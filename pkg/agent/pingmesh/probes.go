// Copyright 2020 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pingmesh

import (
	"net"
	"sync"
	"time"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/go-ping/ping"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/vmware-tanzu/antrea/pkg/agent/config"
)

const (
	controllerName = "PingMeshController"
	// Interval of reprocessing every node.
	nodeResyncPeriod = 60 * time.Second
	// How long to wait before retrying the processing of a node change
	minRetryDelay = 2 * time.Second
	maxRetryDelay = 120 * time.Second
	// Default number of workers processing a node change
	defaultWorkers = 1
)

type nodeLatencyInfo struct {
	sync.Mutex
	gatewayIP []net.IP
	latencies []time.Duration
	stopCh    chan struct{}
}

type PingMeshController struct {
	sync.Mutex
	nodes map[string]*nodeLatencyInfo

	kubeClient       clientset.Interface
	nodeConfig       *config.NodeConfig
	nodeInformer     coreinformers.NodeInformer
	nodeLister       corelisters.NodeLister
	nodeListerSynced cache.InformerSynced
	queue            workqueue.RateLimitingInterface
}

func NewPingMeshController(
	kubeClient clientset.Interface,
	informerFactory informers.SharedInformerFactory,
	nodeConfig *config.NodeConfig,
) *PingMeshController {
	nodeInformer := informerFactory.Core().V1().Nodes()
	controller := &PingMeshController{
		nodes:            make(map[string]*nodeLatencyInfo),
		kubeClient:       kubeClient,
		nodeConfig:       nodeConfig,
		nodeInformer:     nodeInformer,
		nodeLister:       nodeInformer.Lister(),
		nodeListerSynced: nodeInformer.Informer().HasSynced,
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "noderoute"),
	}
	nodeInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(cur interface{}) {
				controller.enqueueNode(cur)
			},
			UpdateFunc: func(old, cur interface{}) {
				controller.enqueueNode(cur)
			},
			DeleteFunc: func(old interface{}) {
				controller.enqueueNode(old)
			},
		},
		nodeResyncPeriod,
	)
	return controller
}

func (c *PingMeshController) enqueueNode(obj interface{}) {
	node, isNode := obj.(*corev1.Node)
	if !isNode {
		deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Received unexpected object: %v", obj)
			return
		}
		node, ok = deletedState.Obj.(*corev1.Node)
		if !ok {
			klog.Errorf("DeletedFinalStateUnknown contains non-Node object: %v", deletedState.Obj)
			return
		}
	}

	// Ignore notifications for this Node
	if node.Name != c.nodeConfig.Name {
		c.queue.Add(node.Name)
	}
}

func (c *PingMeshController) Run(stopCh <-chan struct{}) {
	defer c.queue.ShutDown()

	klog.Infof("Starting %s", controllerName)
	defer klog.Infof("Shutting down %s", controllerName)

	if !cache.WaitForNamedCacheSync(controllerName, stopCh, c.nodeListerSynced) {
		return
	}

	for i := 0; i < defaultWorkers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	// debugging
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				c.Lock()
				for nodeName, info := range c.nodes {
					info.Lock()
					klog.Infof("Latencies %s: %v", nodeName, info.latencies)
					info.Unlock()
				}
				c.Unlock()
			}
		}
	}()

	<-stopCh
}

// worker is a long-running function that will continually call the processNextWorkItem function in
// order to read and process a message on the workqueue.
func (c *PingMeshController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *PingMeshController) processNextWorkItem() bool {
	obj, quit := c.queue.Get()
	if quit {
		return false
	}
	// We call Done here so the workqueue knows we have finished processing this item. We also
	// must remember to call Forget if we do not want this work item being re-queued. For
	// example, we do not call Forget if a transient error occurs, instead the item is put back
	// on the workqueue and attempted again after a back-off period.
	defer c.queue.Done(obj)

	// We expect strings (Node name) to come off the workqueue.
	if key, ok := obj.(string); !ok {
		// As the item in the workqueue is actually invalid, we call Forget here else we'd
		// go into a loop of attempting to process a work item that is invalid.
		// This should not happen: enqueueNode only enqueues strings.
		c.queue.Forget(obj)
		klog.Errorf("Expected string in work queue but got %#v", obj)
		return true
	} else if err := c.syncNodeProbes(key); err == nil {
		// If no error occurs we Forget this item so it does not get queued again until
		// another change happens.
		c.queue.Forget(key)
	} else {
		// Put the item back on the workqueue to handle any transient errors.
		c.queue.AddRateLimited(key)
		klog.Errorf("Error syncing Node %s, requeuing. Error: %v", key, err)
	}
	return true
}

func (c *PingMeshController) syncNodeProbes(nodeName string) error {
	node, err := c.nodeLister.Get(nodeName)
	if err != nil {
		return c.deleteNode(nodeName)
	}
	return c.addNode(nodeName, node)
}

func getPodCIDRsOnNode(node *corev1.Node) []string {
	if node.Spec.PodCIDRs != nil {
		return node.Spec.PodCIDRs
	}

	if node.Spec.PodCIDR == "" {
		klog.Errorf("PodCIDR is empty for Node %s", node.Name)
		// Does not help to return an error and trigger controller retries.
		return nil
	}
	return []string{node.Spec.PodCIDR}
}

func (c *PingMeshController) addNode(nodeName string, node *corev1.Node) error {
	podCIDRStrs := getPodCIDRsOnNode(node)
	if len(podCIDRStrs) == 0 {
		// If no valid PodCIDR is configured in Node.Spec, return immediately.
		return nil
	}

	peerGatewayIPs := make([]net.IP, 0, len(podCIDRStrs))
	for _, podCIDR := range podCIDRStrs {
		if podCIDR == "" {
			klog.Errorf("PodCIDR is empty for Node %s", nodeName)
			// Does not help to return an error and trigger controller retries.
			return nil
		}
		peerPodCIDRAddr, _, err := net.ParseCIDR(podCIDR)
		if err != nil {
			klog.Errorf("Failed to parse PodCIDR %s for Node %s", node.Spec.PodCIDR, nodeName)
			return nil
		}
		peerGatewayIP := ip.NextIP(peerPodCIDRAddr)
		peerGatewayIPs = append(peerGatewayIPs, peerGatewayIP)
	}

	c.Lock()
	defer c.Unlock()
	if _, ok := c.nodes[nodeName]; ok {
		return nil
	}
	c.nodes[nodeName] = &nodeLatencyInfo{
		gatewayIP: peerGatewayIPs,
		latencies: make([]time.Duration, len(peerGatewayIPs)),
		stopCh:    make(chan struct{}),
	}
	c.nodes[nodeName].startProbes()
	return nil
}

func (c *PingMeshController) deleteNode(nodeName string) error {
	c.Lock()
	defer c.Unlock()
	if info, ok := c.nodes[nodeName]; ok {
		info.stopCh <- struct{}{}
		delete(c.nodes, nodeName)
	}
	return nil
}

func (info *nodeLatencyInfo) startProbes() {
	oneProbe := func(peerGatewayIP net.IP, idx int) {
		klog.Infof("Probing %v", peerGatewayIP)
		pinger, err := ping.NewPinger(peerGatewayIP.String())
		pinger.SetPrivileged(true)
		if err != nil {
			panic(err)
		}
		pinger.Count = 1
		pinger.Timeout = 10 * time.Second
		err = pinger.Run() // Blocks until finished.
		if err != nil {
			panic(err)
		}
		stats := pinger.Statistics() // get send/receive/rtt stats
		// klog.Infof("Stats %v", stats)
		info.Lock()
		defer info.Unlock()
		if stats.PacketsRecv == 1 {
			info.latencies[idx] = stats.AvgRtt
		} else {
			info.latencies[idx] = 10 * time.Second
		}
	}
	for idx, peerGatewayIP := range info.gatewayIP {
		go wait.Until(func() { oneProbe(peerGatewayIP, idx) }, 10*time.Second, info.stopCh)
	}
}
