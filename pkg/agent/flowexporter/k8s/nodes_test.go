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
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"antrea.io/antrea/pkg/agent/config"
)

func makeNode(name string, cidrs ...*net.IPNet) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if len(cidrs) == 0 {
		return node
	}
	node.Spec.PodCIDR = cidrs[0].String()
	for _, cidr := range cidrs {
		node.Spec.PodCIDRs = append(node.Spec.PodCIDRs, cidr.String())
	}
	return node
}

var (
	_, podCIDR, _    = net.ParseCIDR("1.1.0.0/24")
	_, podCIDRv6, _  = net.ParseCIDR("2001:4860:0000::/48")
	_, podCIDR1, _   = net.ParseCIDR("1.1.1.0/24")
	_, podCIDR1v6, _ = net.ParseCIDR("2001:4860:0001::/48")

	node = makeNode("node", podCIDR, podCIDRv6)

	nodeConfig = &config.NodeConfig{
		PodIPv4CIDR: podCIDR,
		PodIPv6CIDR: podCIDRv6,
	}
)

func cidrToPrefix(cidr *net.IPNet) netip.Prefix {
	return netip.MustParsePrefix(cidr.String())
}

func setupTest(t testing.TB, startInformer bool, objects ...runtime.Object) (*fake.Clientset, *NodeStore) {
	clientset := fake.NewSimpleClientset(objects...)
	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	s := NewNodeStore(informerFactory.Core().V1().Nodes().Informer(), nodeConfig)
	if startInformer {
		stopCh := make(chan struct{})
		t.Cleanup(func() { close(stopCh) })
		informerFactory.Start(stopCh)
		informerFactory.WaitForCacheSync(stopCh)
	}
	return clientset, s
}

func TestAddNodeHandler(t *testing.T) {
	_, s := setupTest(t, true, node)
	expectedNodesToPodSubnets := map[string][]netip.Prefix{
		"node": {cidrToPrefix(podCIDR), cidrToPrefix(podCIDRv6)},
	}
	expectedPodSubnetsToNodes := map[netip.Prefix][]string{
		cidrToPrefix(podCIDR):   {"node"},
		cidrToPrefix(podCIDRv6): {"node"},
	}
	assert.Equal(t, expectedNodesToPodSubnets, s.nodesToPodSubnets)
	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Equal(t, expectedNodesToPodSubnets, s.nodesToPodSubnets)
		assert.Equal(t, expectedPodSubnetsToNodes, s.podSubnetsToNodes)
	}, 1*time.Second, 10*time.Millisecond)
}

func TestOnNodeUpdate(t *testing.T) {
	_, s := setupTest(t, false)
	oldNode := makeNode("node")
	s.onNodeCreate(oldNode)
	assert.Empty(t, s.nodesToPodSubnets)
	assert.Empty(t, s.podSubnetsToNodes)
	s.onNodeUpdate(oldNode, node)
	expectedNodesToPodSubnets := map[string][]netip.Prefix{
		"node": {cidrToPrefix(podCIDR), cidrToPrefix(podCIDRv6)},
	}
	expectedPodSubnetsToNodes := map[netip.Prefix][]string{
		cidrToPrefix(podCIDR):   {"node"},
		cidrToPrefix(podCIDRv6): {"node"},
	}
	assert.Equal(t, expectedNodesToPodSubnets, s.nodesToPodSubnets)
	assert.Equal(t, expectedPodSubnetsToNodes, s.podSubnetsToNodes)
}

func TestOnNodeDelete(t *testing.T) {
	_, s := setupTest(t, false)
	s.onNodeCreate(node)
	expectedNodesToPodSubnets := map[string][]netip.Prefix{
		"node": {cidrToPrefix(podCIDR), cidrToPrefix(podCIDRv6)},
	}
	expectedPodSubnetsToNodes := map[netip.Prefix][]string{
		cidrToPrefix(podCIDR):   {"node"},
		cidrToPrefix(podCIDRv6): {"node"},
	}
	assert.Equal(t, expectedNodesToPodSubnets, s.nodesToPodSubnets)
	assert.Equal(t, expectedPodSubnetsToNodes, s.podSubnetsToNodes)
	s.onNodeDelete(node)
	assert.Empty(t, s.nodesToPodSubnets)
	assert.Empty(t, s.podSubnetsToNodes)
}

// TestOnNodeDuplicateCIDRs tests the case where the same CIDR is reused, and the Create event for
// the new Node is processed before the Delete event for the old Node.
func TestOnNodeDuplicateCIDRs(t *testing.T) {
	_, s := setupTest(t, false)
	s.onNodeCreate(node)
	// same CIDRs
	node1 := makeNode("node1", podCIDR, podCIDRv6)
	s.onNodeCreate(node1)
	expectedNodesToPodSubnets := map[string][]netip.Prefix{
		"node":  {cidrToPrefix(podCIDR), cidrToPrefix(podCIDRv6)},
		"node1": {cidrToPrefix(podCIDR), cidrToPrefix(podCIDRv6)},
	}
	expectedPodSubnetsToNodes := map[netip.Prefix][]string{
		cidrToPrefix(podCIDR):   {"node", "node1"},
		cidrToPrefix(podCIDRv6): {"node", "node1"},
	}
	assert.Equal(t, expectedNodesToPodSubnets, s.nodesToPodSubnets)
	assert.Equal(t, expectedPodSubnetsToNodes, s.podSubnetsToNodes)
	s.onNodeDelete(node)
	expectedNodesToPodSubnets = map[string][]netip.Prefix{
		"node1": {cidrToPrefix(podCIDR), cidrToPrefix(podCIDRv6)},
	}
	expectedPodSubnetsToNodes = map[netip.Prefix][]string{
		cidrToPrefix(podCIDR):   {"node1"},
		cidrToPrefix(podCIDRv6): {"node1"},
	}
	assert.Equal(t, expectedNodesToPodSubnets, s.nodesToPodSubnets)
	assert.Equal(t, expectedPodSubnetsToNodes, s.podSubnetsToNodes)
}

func TestIPInPodSubnets(t *testing.T) {
	_, s := setupTest(t, false)
	s.onNodeCreate(node)
	t.Run("v4", func(t *testing.T) {
		assert.True(t, s.IPInPodSubnets(netip.MustParseAddr("1.1.0.1")))
		assert.True(t, s.IPInPodSubnets(netip.MustParseAddr("1.1.0.101")))
		assert.False(t, s.IPInPodSubnets(netip.MustParseAddr("1.1.1.101")))
	})
	t.Run("v6", func(t *testing.T) {
		assert.True(t, s.IPInPodSubnets(netip.MustParseAddr("2001:4860:0000::1")))
		assert.True(t, s.IPInPodSubnets(netip.MustParseAddr("2001:4860:0000::101")))
		assert.False(t, s.IPInPodSubnets(netip.MustParseAddr("2001:4860:0001::101")))
	})
}

func TestIPIsGatewayIP(t *testing.T) {
	_, s := setupTest(t, false)
	s.onNodeCreate(node)
	s.onNodeCreate(makeNode("node1", podCIDR1, podCIDR1v6))
	t.Run("v4", func(t *testing.T) {
		assert.True(t, s.IPIsGatewayIP(netip.MustParseAddr("1.1.0.1")))
		assert.False(t, s.IPIsGatewayIP(netip.MustParseAddr("1.1.0.101")))
		assert.False(t, s.IPIsGatewayIP(netip.MustParseAddr("1.1.1.101")))
		assert.True(t, s.IPIsGatewayIP(netip.MustParseAddr("1.1.1.1")))
		assert.False(t, s.IPIsGatewayIP(netip.MustParseAddr("1.1.2.1")))
	})
	t.Run("v6", func(t *testing.T) {
		assert.True(t, s.IPIsGatewayIP(netip.MustParseAddr("2001:4860:0000::1")))
		assert.False(t, s.IPIsGatewayIP(netip.MustParseAddr("2001:4860:0000::101")))
		assert.False(t, s.IPIsGatewayIP(netip.MustParseAddr("2001:4860:0001::101")))
		assert.True(t, s.IPIsGatewayIP(netip.MustParseAddr("2001:4860:0001::1")))
		assert.False(t, s.IPIsGatewayIP(netip.MustParseAddr("2001:4860:0002::1")))
	})
}
func BenchmarkIPInPodSubnets(b *testing.B) {
	_, s := setupTest(b, false)
	s.onNodeCreate(node)
	b.ResetTimer()
	for range b.N {
		assert.True(b, s.IPInPodSubnets(netip.MustParseAddr("1.1.0.101")))
		assert.False(b, s.IPInPodSubnets(netip.MustParseAddr("1.1.1.101")))
	}
}
