// Copyright 2025 Antrea Authors
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

package k8s

import (
	"fmt"
	"net/netip"
	"slices"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"antrea.io/antrea/pkg/agent/config"
	"antrea.io/antrea/pkg/agent/util"
)

type NodeStore struct {
	sync.Mutex
	// Ideally we would just want a set here, keeping track of current Pod subnets.
	// However, because a PodCIDR can be reused (when an old Node is deleted and a new one is
	// created), and there is no guarantee about the order in which the events will be
	// processed, we need more complex data structures to "ref count" the subnets.
	// We also do not want to use an Indexer as we want to avoid string conversions.
	nodesToPodSubnets map[string][]netip.Prefix
	podSubnetsToNodes map[netip.Prefix][]string
	maskSizeV4        int
	maskSizeV6        int
}

func NewNodeStore(nodeInformer cache.SharedIndexInformer, nodeConfig *config.NodeConfig) *NodeStore {
	var maskSizeV4, maskSizeV6 int
	if nodeConfig.PodIPv4CIDR != nil {
		maskSizeV4, _ = nodeConfig.PodIPv4CIDR.Mask.Size()
	}
	if nodeConfig.PodIPv6CIDR != nil {
		maskSizeV6, _ = nodeConfig.PodIPv6CIDR.Mask.Size()
	}
	s := &NodeStore{
		nodesToPodSubnets: make(map[string][]netip.Prefix),
		podSubnetsToNodes: make(map[netip.Prefix][]string),
		maskSizeV4:        maskSizeV4,
		maskSizeV6:        maskSizeV6,
	}
	nodeInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    s.onNodeCreate,
			UpdateFunc: s.onNodeUpdate,
			DeleteFunc: s.onNodeDelete,
		},
	)

	return s
}

func (s *NodeStore) onNodeCreate(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.ErrorS(nil, "Received unexpected object", "obj", obj)
		return
	}
	subnets, err := getPodSubnetsForNode(node)
	if err != nil {
		klog.ErrorS(err, "Invalid CIDRs for Node", "node", klog.KObj(node))
		return
	}
	if len(subnets) == 0 {
		return
	}
	s.Lock()
	defer s.Unlock()
	s.nodesToPodSubnets[node.Name] = subnets
	for _, subnet := range subnets {
		s.podSubnetsToNodes[subnet] = append(s.podSubnetsToNodes[subnet], node.Name)
	}
}

func (s *NodeStore) onNodeUpdate(oldObj, newObj interface{}) {
	node, ok := newObj.(*corev1.Node)
	if !ok {
		klog.ErrorS(nil, "Received unexpected object", "obj", newObj)
		return
	}
	subnets, err := getPodSubnetsForNode(node)
	if err != nil {
		klog.ErrorS(err, "Invalid CIDRs for Node", "node", klog.KObj(node))
		return
	}
	s.Lock()
	defer s.Unlock()
	// CIDRs cannot be updated once set, so skip if Node is already present in map.
	if _, ok := s.nodesToPodSubnets[node.Name]; ok {
		return
	}
	s.nodesToPodSubnets[node.Name] = subnets
	for _, subnet := range subnets {
		s.podSubnetsToNodes[subnet] = append(s.podSubnetsToNodes[subnet], node.Name)
	}
}

func (s *NodeStore) onNodeDelete(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.ErrorS(nil, "Received unexpected object", "obj", obj)
			return
		}
		node, ok = deletedState.Obj.(*corev1.Node)
		if !ok {
			klog.ErrorS(nil, "DeletedFinalStateUnknown contains non-Node object", "obj", deletedState.Obj)
			return
		}
	}
	subnets, ok := s.nodesToPodSubnets[node.Name]
	if !ok {
		return
	}
	s.Lock()
	defer s.Unlock()
	delete(s.nodesToPodSubnets, node.Name)
	for _, subnet := range subnets {
		s.podSubnetsToNodes[subnet] = slices.DeleteFunc(s.podSubnetsToNodes[subnet], func(s string) bool {
			return s == node.Name
		})
		if len(s.podSubnetsToNodes[subnet]) == 0 {
			delete(s.podSubnetsToNodes, subnet)
		}
	}
}

func getPodSubnetsForNode(node *corev1.Node) ([]netip.Prefix, error) {
	if node.Spec.PodCIDRs != nil {
		subnets := make([]netip.Prefix, len(node.Spec.PodCIDRs))
		for idx := range node.Spec.PodCIDRs {
			prefix, err := netip.ParsePrefix(node.Spec.PodCIDRs[idx])
			if err != nil {
				return nil, fmt.Errorf("invalid entry in PodCIDRs for Node '%s': %w", node.Name, err)
			}
			// The prefix should already be masked because of how K8s populates the
			// PodCIDRs field, but there is no harm in being extra careful here.
			subnets[idx] = prefix.Masked()
		}
		return subnets, nil
	}
	if node.Spec.PodCIDR == "" {
		klog.V(3).InfoS("No PodCIDR for Node", "node", klog.KObj(node))
		return nil, nil
	}
	prefix, err := netip.ParsePrefix(node.Spec.PodCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid PodCIDR for Node '%s': %w", node.Name, err)
	}
	return []netip.Prefix{prefix.Masked()}, nil
}

func (s *NodeStore) findPodSubnetForIP(ip netip.Addr) (netip.Prefix, bool) {
	var maskSize int
	if ip.Is4() {
		maskSize = s.maskSizeV4
	} else {
		maskSize = s.maskSizeV6
	}
	if maskSize == 0 {
		return netip.Prefix{}, false
	}
	prefix, _ := ip.Prefix(maskSize)
	s.Lock()
	defer s.Unlock()
	_, ok := s.podSubnetsToNodes[prefix]
	return prefix, ok
}

func (s *NodeStore) IPInPodSubnets(ip netip.Addr) bool {
	_, ok := s.findPodSubnetForIP(ip)
	return ok
}

func (s *NodeStore) IPIsGatewayIP(ip netip.Addr) bool {
	prefix, ok := s.findPodSubnetForIP(ip)
	if !ok {
		return false
	}
	return ip == util.GetGatewayIPForPodPrefix(prefix)
}
