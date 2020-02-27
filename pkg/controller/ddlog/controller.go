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

// Package networkpolicy provides NetworkPolicyController implementation to manage
// and synchronize the Pods and Namespaces affected by Network Policies and enforce
// their rules.

package ddlog

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	networkinginformers "k8s.io/client-go/informers/networking/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	networkinglisters "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/vmware-tanzu/antrea/pkg/apis/networking"
	"github.com/vmware-tanzu/antrea/pkg/apiserver/storage"
	antreatypes "github.com/vmware-tanzu/antrea/pkg/controller/types"
	"github.com/vmware-tanzu/antrea/pkg/ddlogk8s"
	"github.com/vmware-tanzu/antrea/pkg/k8s"
	"github.com/vmware/differential-datalog/go/pkg/ddlog"
)

const (
	// Interval of synchronizing status from apiserver.
	syncPeriod = 0

	// How long to wait before retrying the processing of a change.
	minRetryDelay = 1 * time.Second
	maxRetryDelay = 300 * time.Second

	defaultInputWorkers = 1

	maxUpdatesPerTransaction = 32
	maxTransactionDelay      = 100 * time.Millisecond
)

// Controller is responsible for synchronizing the Namespaces and Pods
// affected by a Network Policy.
type Controller struct {
	kubeClient  clientset.Interface
	podInformer coreinformers.PodInformer

	// podLister is able to list/get Pods and is populated by the shared informer passed to
	// NewNetworkPolicyController.
	podLister corelisters.PodLister

	// podListerSynced is a function which returns true if the Pod shared informer has been synced at least once.
	podListerSynced cache.InformerSynced

	namespaceInformer coreinformers.NamespaceInformer

	// namespaceLister is able to list/get Namespaces and is populated by the shared informer passed to
	// NewNetworkPolicyController.
	namespaceLister corelisters.NamespaceLister

	// namespaceListerSynced is a function which returns true if the Namespace shared informer has been synced at least once.
	namespaceListerSynced cache.InformerSynced

	networkPolicyInformer networkinginformers.NetworkPolicyInformer

	// networkPolicyLister is able to list/get Network Policies and is populated by the shared informer passed to
	// NewNetworkPolicyController.
	networkPolicyLister networkinglisters.NetworkPolicyLister

	// networkPolicyListerSynced is a function which returns true if the Network Policy shared informer has been synced at least once.
	networkPolicyListerSynced cache.InformerSynced

	podQueue workqueue.RateLimitingInterface

	namespaceQueue workqueue.RateLimitingInterface

	networkPolicyQueue workqueue.RateLimitingInterface

	ddlogProgram *ddlog.Program

	ddlogUpdatesCh chan ddlog.Command

	// addressGroupStore is the storage where the populated Address Groups are stored.
	addressGroupStore storage.Interface

	// appliedToGroupStore is the storage where the populated AppliedTo Groups are stored.
	appliedToGroupStore storage.Interface

	// internalNetworkPolicyStore is the storage where the populated internal Network Policy are stored.
	internalNetworkPolicyStore storage.Interface
}

// NewController returns a new *Controller.
func NewController(
	kubeClient clientset.Interface,
	podInformer coreinformers.PodInformer,
	namespaceInformer coreinformers.NamespaceInformer,
	networkPolicyInformer networkinginformers.NetworkPolicyInformer,
	ddlogProgram *ddlog.Program,
	addressGroupStore storage.Interface,
	appliedToGroupStore storage.Interface,
	internalNetworkPolicyStore storage.Interface,
) *Controller {
	c := &Controller{
		kubeClient:                 kubeClient,
		podInformer:                podInformer,
		podLister:                  podInformer.Lister(),
		podListerSynced:            podInformer.Informer().HasSynced,
		namespaceInformer:          namespaceInformer,
		namespaceLister:            namespaceInformer.Lister(),
		namespaceListerSynced:      namespaceInformer.Informer().HasSynced,
		networkPolicyInformer:      networkPolicyInformer,
		networkPolicyLister:        networkPolicyInformer.Lister(),
		networkPolicyListerSynced:  networkPolicyInformer.Informer().HasSynced,
		ddlogProgram:               ddlogProgram,
		podQueue:                   workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "pods"),
		namespaceQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "namespaces"),
		networkPolicyQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "networkPolicies"),
		ddlogUpdatesCh:             make(chan ddlog.Command, maxUpdatesPerTransaction),
		addressGroupStore:          addressGroupStore,
		appliedToGroupStore:        appliedToGroupStore,
		internalNetworkPolicyStore: internalNetworkPolicyStore,
	}
	// Add handlers for Pod events.
	podInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.enqueuePod,
			UpdateFunc: func(oldObj, curObj interface{}) { c.enqueuePod(curObj) },
			DeleteFunc: c.enqueuePod,
		},
		syncPeriod,
	)
	// Add handlers for Namespace events.
	namespaceInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.enqueueNamespace,
			UpdateFunc: func(oldObj, curObj interface{}) { c.enqueueNamespace(curObj) },
			DeleteFunc: c.enqueueNamespace,
		},
		syncPeriod,
	)
	// Add handlers for NetworkPolicy events.
	networkPolicyInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.enqueueNetworkPolicy,
			UpdateFunc: func(oldObj, curObj interface{}) { c.enqueueNetworkPolicy(curObj) },
			DeleteFunc: c.enqueueNetworkPolicy,
		},
		syncPeriod,
	)
	return c
}

func (c *Controller) enqueuePod(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Error when generating key for Pod: %v", err)
		return
	}
	c.podQueue.Add(key)
}

func (c *Controller) enqueueNamespace(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Error when generating key for Namespace: %v", err)
		return
	}
	c.namespaceQueue.Add(key)
}

func (c *Controller) enqueueNetworkPolicy(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Error when generating key for NetworkPolicy: %v", err)
		return
	}
	c.networkPolicyQueue.Add(key)
}

func dumpOneStore(store storage.Interface, name string) {
	f, _ := os.Create(name)
	for _, v := range store.List() {
		spew.Fprintf(f, "%#v\n", v)
	}
	f.Sync()
}

func (c *Controller) dumpStore() {
	dumpOneStore(c.internalNetworkPolicyStore, "/var/run/antrea-ddlog/np.txt")
	dumpOneStore(c.addressGroupStore, "/var/run/antrea-ddlog/address-group.txt")
	dumpOneStore(c.appliedToGroupStore, "/var/run/antrea-ddlog/applied-to.txt")
}

func (c *Controller) dumpStoreUntil(stopCh <-chan struct{}) {
	wait.Until(c.dumpStore, time.Second, stopCh)
}

func toGroupSelector(record ddlog.Record) *antreatypes.GroupSelector {
	r := record.AsStruct()
	rName := r.At(0)
	rNamespaceSelector := r.At(1).AsStruct()
	rPodSelector := r.At(2).AsStruct()

	var namespaceSelector *metav1.LabelSelector
	var namespace string
	switch rNamespaceSelector.Name() {
	case "types_NSSelectorNS":
		namespace = rNamespaceSelector.At(0).ToString()
	case "types_NSSelectorLS":
		namespaceSelector = ddlogk8s.RecordToLabelSelector(rNamespaceSelector.At(0))
	default:
		klog.Errorf("Not a valid namespace selector %s", rNamespaceSelector.Name())
	}

	var podSelector *metav1.LabelSelector
	if rPodSelector.Name() == "std_Some" {
		podSelector = ddlogk8s.RecordToLabelSelector(rPodSelector.At(0))
	}

	return &antreatypes.GroupSelector{
		NormalizedName:    rName.ToString(),
		Namespace:         namespace,
		PodSelector:       podSelector,
		NamespaceSelector: namespaceSelector,
	}
}

func toSpanMeta(record ddlog.Record) *antreatypes.SpanMeta {
	r := record.AsSet()
	spanMeta := &antreatypes.SpanMeta{
		NodeNames: sets.NewString(),
	}
	for i := 0; i < r.Size(); i++ {
		spanMeta.NodeNames.Insert(r.At(i).ToString())
	}
	return spanMeta
}

func toPodReference(record ddlog.Record) networking.PodReference {
	r := record.AsStruct()
	return networking.PodReference{
		Name:      r.At(0).ToString(),
		Namespace: r.At(1).ToString(),
	}
}

func toPodSet(record ddlog.Record) antreatypes.PodSet {
	r := record.AsSet()
	podSet := antreatypes.PodSet{}
	for i := 0; i < r.Size(); i++ {
		podSet.Insert(toPodReference(r.At(i)))
	}
	return podSet
}

func (c *Controller) handleAppliedToGroup(record ddlog.Record, polarity ddlog.OutPolarity) {
	r, err := record.AsStructSafe()
	if err != nil {
		klog.Errorf("Record is not a struct")
		return
	}
	rDescr := r.At(0).AsStruct()
	rPodsByNode := r.At(1).AsMap()

	podsByNode := make(map[string]antreatypes.PodSet)
	for i := 0; i < rPodsByNode.Size(); i++ {
		rKey, rValue := rPodsByNode.At(i)
		podsByNode[rKey.ToString()] = toPodSet(rValue)
	}

	appliedToGroup := &antreatypes.AppliedToGroup{
		UID:        ddlogk8s.RecordToUID(rDescr.At(0)),
		Name:       rDescr.At(1).ToString(),
		Selector:   *toGroupSelector(rDescr.At(2)),
		PodsByNode: podsByNode,
		SpanMeta:   *toSpanMeta(r.At(2)),
	}
	if polarity == ddlog.OutPolarityInsert {
		c.appliedToGroupStore.Create(appliedToGroup)
	} else {
		key := appliedToGroup.Name
		c.appliedToGroupStore.Delete(key)
	}
}

func (c *Controller) handleAddressGroup(record ddlog.Record, polarity ddlog.OutPolarity) {
	r, err := record.AsStructSafe()
	if err != nil {
		klog.Errorf("Record is not a struct")
		return
	}
	rDescr := r.At(0).AsStruct()
	rAddresses := r.At(1).AsSet()

	addresses := sets.NewString()
	for i := 0; i < rAddresses.Size(); i++ {
		addresses.Insert(rAddresses.At(i).ToString())
	}

	addressGroup := &antreatypes.AddressGroup{
		UID:       ddlogk8s.RecordToUID(rDescr.At(0)),
		Name:      rDescr.At(1).ToString(),
		Selector:  *toGroupSelector(rDescr.At(2)),
		Addresses: addresses,
		SpanMeta:  *toSpanMeta(r.At(2)),
	}
	if polarity == ddlog.OutPolarityInsert {
		c.addressGroupStore.Create(addressGroup)
	} else {
		key := addressGroup.Name
		c.addressGroupStore.Delete(key)
	}
}

func toDirection(record ddlog.Record) networking.Direction {
	r := record.AsStruct()
	switch r.Name() {
	case "types_DirectionIn":
		return networking.DirectionIn
	case "types_DirectionOut":
		return networking.DirectionOut
	default:
		klog.Errorf("Not a valid direction %s", r.Name())
	}
	return networking.Direction("")
}

func toService(record ddlog.Record) *networking.Service {
	r := record.AsStruct()
	rProtocol := r.At(0)
	rPort := r.At(1).AsStruct()

	var protocol *networking.Protocol
	// for some reason the DDlog program is using Protocol here and not
	// Option<Protocol>. Maybe it just defaults to TCP.
	p := networking.Protocol(rProtocol.ToString())
	protocol = &p

	var port *int32
	if rPort.Name() == "std_Some" {
		p := rPort.At(0).ToI32()
		port = &p
	}

	return &networking.Service{
		Protocol: protocol,
		Port:     port,
	}
}

func toIPAddress(record ddlog.Record) networking.IPAddress {
	r := record.AsStruct().At(0).AsVector()
	address := make([]byte, r.Size())
	for i := 0; i < r.Size(); i++ {
		address[i] = byte(r.At(i).ToU32())
	}
	return address
}

func toIPNet(record ddlog.Record) *networking.IPNet {
	r := record.AsStruct()
	return &networking.IPNet{
		IP:           toIPAddress(r.At(0)),
		PrefixLength: r.At(1).ToI32(),
	}
}

func toIPBlock(record ddlog.Record) *networking.IPBlock {
	r := record.AsStruct()
	rExcept := r.At(1).AsVector()

	except := make([]networking.IPNet, rExcept.Size())
	for i := 0; i < rExcept.Size(); i++ {
		except[i] = *toIPNet(rExcept.At(i))
	}

	return &networking.IPBlock{
		CIDR:   *toIPNet(r.At(0)),
		Except: except,
	}
}

func toNetworkPolicyPeer(record ddlog.Record) *networking.NetworkPolicyPeer {
	r := record.AsStruct()
	rAddressGroups := r.At(0).AsVector()
	rIPBlocks := r.At(1).AsVector()

	// The current Antrea controller code uses nil slices and not empty slices for these

	// addressGroups := make([]string, rAddressGroups.Size())
	// for i := 0; i < rAddressGroups.Size(); i++ {
	// 	addressGroups[i] = rAddressGroups.At(i).ToString()
	// }

	// ipBlocks := make([]networking.IPBlock, rIPBlocks.Size())
	// for i := 0; i < rIPBlocks.Size(); i++ {
	// 	ipBlocks[i] = *toIPBlock(rIPBlocks.At(i))
	// }

	var addressGroups []string
	for i := 0; i < rAddressGroups.Size(); i++ {
		addressGroups = append(addressGroups, rAddressGroups.At(i).ToString())
	}

	var ipBlocks []networking.IPBlock
	for i := 0; i < rIPBlocks.Size(); i++ {
		ipBlocks = append(ipBlocks, *toIPBlock(rIPBlocks.At(i)))
	}

	return &networking.NetworkPolicyPeer{
		AddressGroups: addressGroups,
		IPBlocks:      ipBlocks,
	}
}

func toNetworkPolicyRule(record ddlog.Record) *networking.NetworkPolicyRule {
	r := record.AsStruct()

	rServices := r.At(3).AsVector()
	// The current Antrea controller code uses a nil slice and not an empty slice
	// services := make([]networking.Service, rServices.Size())
	// for i := 0; i < rServices.Size(); i++ {
	// 	services[i] = *toService(rServices.At(i))
	// }
	var services []networking.Service
	for i := 0; i < rServices.Size(); i++ {
		services = append(services, *toService(rServices.At(i)))
	}

	return &networking.NetworkPolicyRule{
		Direction: toDirection(r.At(0)),
		From:      *toNetworkPolicyPeer(r.At(1)),
		To:        *toNetworkPolicyPeer(r.At(2)),
		Services:  services,
	}
}

func (c *Controller) handleNetworkPolicyOut(record ddlog.Record, polarity ddlog.OutPolarity) {
	r, err := record.AsStructSafe()
	if err != nil {
		klog.Errorf("Record is not a struct")
		return
	}
	rDescr := r.At(0).AsStruct()
	rRules := rDescr.At(3).AsVector()
	rAppliedToGroups := rDescr.At(4).AsVector()

	rules := make([]networking.NetworkPolicyRule, rRules.Size())
	for i := 0; i < rRules.Size(); i++ {
		rules[i] = *toNetworkPolicyRule(rRules.At(i))
	}

	appliedToGroups := make([]string, rAppliedToGroups.Size())
	for i := 0; i < rAppliedToGroups.Size(); i++ {
		appliedToGroups[i] = rAppliedToGroups.At(i).ToString()
	}

	networkPolicy := &antreatypes.NetworkPolicy{
		UID:             ddlogk8s.RecordToUID(rDescr.At(0)),
		Name:            rDescr.At(1).ToString(),
		Namespace:       rDescr.At(2).ToString(),
		Rules:           rules,
		AppliedToGroups: appliedToGroups,
		SpanMeta:        *toSpanMeta(r.At(1)),
	}
	if polarity == ddlog.OutPolarityInsert {
		c.internalNetworkPolicyStore.Create(networkPolicy)
	} else {
		key := k8s.NamespacedName(networkPolicy.Namespace, networkPolicy.Name)
		c.internalNetworkPolicyStore.Delete(key)
	}
}

func (c *Controller) HandleRecordOut(tableID ddlog.TableID, record ddlog.Record, polarity ddlog.OutPolarity) {
	klog.Infof("DDlog output record: %s:%s:%s", ddlog.GetTableName(tableID), record.Dump(), polarity)

	switch tableID {
	case ddlogk8s.AppliedToGroupTableID:
		c.handleAppliedToGroup(record, polarity)
	case ddlogk8s.AddressGroupTableID:
		c.handleAddressGroup(record, polarity)
	case ddlogk8s.NetworkPolicyOutTableID:
		c.handleNetworkPolicyOut(record, polarity)
	default:
		klog.Errorf("Unknown output table ID: %d", tableID)
	}
}

func (c *Controller) Run(stopCh <-chan struct{}) {
	defer c.podQueue.ShutDown()
	defer c.namespaceQueue.ShutDown()
	defer c.networkPolicyQueue.ShutDown()

	go c.dumpStoreUntil(stopCh)

	klog.Info("Starting controller")
	defer klog.Info("Shutting down controller")

	klog.Info("Waiting for caches to sync for controller")
	if !cache.WaitForCacheSync(stopCh, c.podListerSynced, c.namespaceListerSynced, c.networkPolicyListerSynced) {
		klog.Error("Unable to sync caches for controller")
		return
	}
	klog.Info("Caches are synced for controller")

	// one worker is in charge of all the transactions since DDLog does not support concurrent
	// transactions
	go c.generateTransactions(stopCh)

	for i := 0; i < defaultInputWorkers; i++ {
		go wait.Until(c.podWorker, time.Second, stopCh)
		go wait.Until(c.namespaceWorker, time.Second, stopCh)
		go wait.Until(c.networkPolicyWorker, time.Second, stopCh)
	}

	<-stopCh
}

// We assume that there cannot be transient issues with DDLog transactions, and so there is no point
// in retrying.
func (c *Controller) generateTransactions(stopCh <-chan struct{}) {
	transactionSize := 0
	parentCxt, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	ctx := parentCxt
	var cancel context.CancelFunc

	commitTransaction := func() {
		transactionSize = 0
		ctx = parentCxt
		if err := c.ddlogProgram.CommitTransaction(); err != nil {
			klog.Errorf("Error when committing DDLog transaction: %v", err)
		}
	}

	handleCommand := func(cmd ddlog.Command) {
		klog.V(2).Infof("Handling command")
		if transactionSize == 0 {
			// start transaction
			if err := c.ddlogProgram.StartTransaction(); err != nil {
				klog.Errorf("Error when starting DDLog transaction: %v", err)
				return
			}
			ctx, cancel = context.WithTimeout(parentCxt, maxTransactionDelay)
		}
		// add to transaction
		if err := c.ddlogProgram.ApplyUpdates(cmd); err != nil {
			klog.Errorf("Error when applying updates with DDLog: %v", err)
			return
		}
		transactionSize++
		if transactionSize >= maxUpdatesPerTransaction {
			cancel()
			commitTransaction()
		}
	}

	for {
		select {
		case cmd := <-c.ddlogUpdatesCh:
			handleCommand(cmd)
		case <-ctx.Done():
			commitTransaction()
		case <-stopCh:
			return
		}
	}
}

func (c *Controller) podWorker() {
	for c.processNextPod() {
	}
}

func (c *Controller) namespaceWorker() {
	for c.processNextNamespace() {
	}
}

func (c *Controller) networkPolicyWorker() {
	for c.processNextNetworkPolicy() {
	}
}

func (c *Controller) processNextPod() bool {
	obj, quit := c.podQueue.Get()
	if quit {
		return false
	}
	defer c.podQueue.Done(obj)
	key := obj.(string)
	if err := c.processPod(key); err != nil {
		klog.Errorf("Error when processing Pod '%s': %v", key, err)
		c.podQueue.AddRateLimited(obj)
		return true
	}
	c.podQueue.Forget(obj)
	return true
}

func (c *Controller) processPod(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("error when extracting Pod namespace and name: %v", err)
	}
	pod, err := c.podLister.Pods(namespace).Get(name)
	var cmd ddlog.Command
	if err != nil { // deletion
		r := ddlogk8s.NewRecordPodKey(namespace, name)
		klog.Infof("DELETE POD: %s", r.Dump())
		cmd = ddlog.NewDeleteKeyCommand(ddlogk8s.PodTableID, r)
	} else {
		r := ddlogk8s.NewRecordPod(pod)
		klog.Infof("UPDATE POD: %s", r.Dump())
		cmd = ddlog.NewInsertOrUpdateCommand(ddlogk8s.PodTableID, r)
	}
	c.ddlogUpdatesCh <- cmd
	return nil
}

func (c *Controller) processNextNamespace() bool {
	obj, quit := c.namespaceQueue.Get()
	if quit {
		return false
	}
	defer c.namespaceQueue.Done(obj)
	key := obj.(string)
	if err := c.processNamespace(key); err != nil {
		klog.Errorf("Error when processing Namespace '%s': %v", key, err)
		c.namespaceQueue.AddRateLimited(obj)
		return true
	}
	c.namespaceQueue.Forget(obj)
	return true
}

func (c *Controller) processNamespace(key string) error {
	namespace, err := c.namespaceLister.Get(key)
	var cmd ddlog.Command
	if err != nil { // deletion
		r := ddlogk8s.NewRecordNamespaceKey(key)
		klog.Infof("DELETE NAMESPACE: %s", r.Dump())
		cmd = ddlog.NewDeleteKeyCommand(ddlogk8s.NamespaceTableID, r)
	} else {
		r := ddlogk8s.NewRecordNamespace(namespace)
		klog.Infof("UPDATE NAMESPACE: %s", r.Dump())
		cmd = ddlog.NewInsertOrUpdateCommand(ddlogk8s.NamespaceTableID, r)
	}
	c.ddlogUpdatesCh <- cmd
	return nil
}

func (c *Controller) processNextNetworkPolicy() bool {
	obj, quit := c.networkPolicyQueue.Get()
	if quit {
		return false
	}
	defer c.networkPolicyQueue.Done(obj)
	key := obj.(string)
	if err := c.processNetworkPolicy(key); err != nil {
		klog.Errorf("Error when processing NetworkPolicy '%s': %v", key, err)
		c.networkPolicyQueue.AddRateLimited(obj)
		return true
	}
	c.networkPolicyQueue.Forget(obj)
	return true
}

func (c *Controller) processNetworkPolicy(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("error when extracting NetworkPolicy namespace and name: %v", err)
	}
	networkPolicy, err := c.networkPolicyLister.NetworkPolicies(namespace).Get(name)
	var cmd ddlog.Command
	if err != nil { // deletion
		r := ddlogk8s.NewRecordNetworkPolicyKey(namespace, name)
		klog.Infof("DELETE NETWORKPOLICY: %s", r.Dump())
		cmd = ddlog.NewDeleteKeyCommand(ddlogk8s.NetworkPolicyTableID, r)
	} else {
		r := ddlogk8s.NewRecordNetworkPolicy(networkPolicy)
		klog.Infof("UPDATE NETWORKPOLICY: %s", r.Dump())
		cmd = ddlog.NewInsertOrUpdateCommand(ddlogk8s.NetworkPolicyTableID, r)
	}
	c.ddlogUpdatesCh <- cmd
	return nil
}
