package testing

import (
	"fmt"
	"io/ioutil"
	"runtime"
	"time"

	"github.com/google/uuid"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
)

func GenControllerTestInputs() (
	[]*v1.Namespace,
	[]*v1.Pod,
	[]*networkingv1.NetworkPolicy,
) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNamespace",
			UID:  "testNamespaceUID",
		},
	}

	numNodes := 200
	numPodsPerNode := 50
	numPods := numNodes * numPodsPerNode
	pods := make([]*v1.Pod, numPods)
	for i := 0; i < numNodes; i++ {
		nodeName := fmt.Sprintf("testNode-%d", i)
		for j := 0; j < numPodsPerNode; j++ {
			idx := i*numPodsPerNode + j
			podName := fmt.Sprintf("testPod-%d", idx)
			podIP := fmt.Sprintf("10.10.%d.%d", i, j)
			p := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: "testNamespace",
					UID:       types.UID(podName),
					Labels:    map[string]string{"pod-name": podName},
				},
				Spec: v1.PodSpec{
					NodeName: nodeName,
				},
				Status: v1.PodStatus{
					PodIP: podIP,
				},
			}
			pods[idx] = p
		}
	}

	newProtocol := func(protocol v1.Protocol) *v1.Protocol {
		return &protocol
	}
	newPort := func(port int) *intstr.IntOrString {
		p := intstr.FromInt(port)
		return &p
	}

	nps := make([]*networkingv1.NetworkPolicy, numPods)
	for i := 0; i < numPods; i++ {
		podName := fmt.Sprintf("testPod-%d", i)
		nextPodName := fmt.Sprintf("testPod-%d", (i+1)%numPods)
		name := fmt.Sprintf("testNP-%d", i)
		np := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "testNamespace",
				UID:       types.UID(name),
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"pod-name": podName},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkingv1.NetworkPolicyIngressRule{
						Ports: []networkingv1.NetworkPolicyPort{
							networkingv1.NetworkPolicyPort{
								Protocol: newProtocol(v1.ProtocolTCP),
								Port:     newPort(80),
							},
						},
						From: []networkingv1.NetworkPolicyPeer{
							networkingv1.NetworkPolicyPeer{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"pod-name": nextPodName},
								},
							},
						},
					},
				},
			},
		}
		nps[i] = np
	}

	return []*v1.Namespace{ns}, pods, nps
}

func GenControllerTestInputs2() (
	[]*v1.Namespace,
	[]*v1.Pod,
	[]*networkingv1.NetworkPolicy,
) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNamespace",
			UID:  "testNamespaceUID",
		},
	}

	numNodes := 2000
	numPodsPerNode := 50
	numPods := numNodes * numPodsPerNode
	pods := make([]*v1.Pod, numPods)
	for i := 0; i < numNodes; i++ {
		nodeName := fmt.Sprintf("testNode-%d", i)
		for j := 0; j < numPodsPerNode; j++ {
			idx := i*numPodsPerNode + j
			var groupName string
			if idx%2 == 0 {
				groupName = "even"
			} else {
				groupName = "odd"
			}
			podName := fmt.Sprintf("testPod-%d", idx)
			podIP := fmt.Sprintf("10.10.%d.%d", i, j)
			p := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: "testNamespace",
					UID:       types.UID(podName),
					Labels: map[string]string{
						"pod-name": podName,
						"group":    groupName,
					},
				},
				Spec: v1.PodSpec{
					NodeName: nodeName,
				},
				Status: v1.PodStatus{
					PodIP: podIP,
				},
			}
			pods[idx] = p
		}
	}

	newProtocol := func(protocol v1.Protocol) *v1.Protocol {
		return &protocol
	}
	newPort := func(port int) *intstr.IntOrString {
		p := intstr.FromInt(port)
		return &p
	}

	nps := make([]*networkingv1.NetworkPolicy, 0)
	addNP := func(appliedTo string, from string) {
		name := fmt.Sprintf("testNP-%s-%s", appliedTo, from)
		np := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "testNamespace",
				UID:       types.UID(name),
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"group": appliedTo},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkingv1.NetworkPolicyIngressRule{
						Ports: []networkingv1.NetworkPolicyPort{
							networkingv1.NetworkPolicyPort{
								Protocol: newProtocol(v1.ProtocolTCP),
								Port:     newPort(80),
							},
						},
						From: []networkingv1.NetworkPolicyPeer{
							networkingv1.NetworkPolicyPeer{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"group": from},
								},
							},
						},
					},
				},
			},
		}
		nps = append(nps, np)
	}
	addNP("even", "odd")
	addNP("odd", "even")

	return []*v1.Namespace{ns}, pods, nps
}

func GenControllerTestInputs3() (
	[]*v1.Namespace,
	[]*v1.Pod,
	[]*networkingv1.NetworkPolicy,
) {
	getObjects := func() ([]*v1.Namespace, []*networkingv1.NetworkPolicy, []*v1.Pod) {
		namespace := rand.String(8)
		namespaces := []*v1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"app": namespace}},
			},
		}
		networkPolicies := []*networkingv1.NetworkPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "default-deny-all", UID: types.UID(uuid.New().String())},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "np-1", UID: types.UID(uuid.New().String())},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app-1": "scale-1"}},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{"app-1": "scale-1"},
									},
								},
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "np-2", UID: types.UID(uuid.New().String())},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app-2": "scale-2"}},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{"app-2": "scale-2"},
									},
								},
							},
						},
					},
				},
			},
		}
		pods := []*v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod1", UID: types.UID(uuid.New().String()), Labels: map[string]string{"app-1": "scale-1"}},
				Spec:       v1.PodSpec{NodeName: getRandomNodeName()},
				Status:     v1.PodStatus{PodIP: getRandomIP()},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod2", UID: types.UID(uuid.New().String()), Labels: map[string]string{"app-1": "scale-1"}},
				Spec:       v1.PodSpec{NodeName: getRandomNodeName()},
				Status:     v1.PodStatus{PodIP: getRandomIP()},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod3", UID: types.UID(uuid.New().String()), Labels: map[string]string{"app-2": "scale-2"}},
				Spec:       v1.PodSpec{NodeName: getRandomNodeName()},
				Status:     v1.PodStatus{PodIP: getRandomIP()},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod4", UID: types.UID(uuid.New().String()), Labels: map[string]string{"app-2": "scale-2"}},
				Spec:       v1.PodSpec{NodeName: getRandomNodeName()},
				Status:     v1.PodStatus{PodIP: getRandomIP()},
			},
		}
		return namespaces, networkPolicies, pods
	}
	namespaces, nps, pods := getXObjects(25000, getObjects)

	return namespaces, pods, nps
}

func GenControllerTestInputs4() (
	[]*v1.Namespace,
	[]*v1.Pod,
	[]*networkingv1.NetworkPolicy,
) {
	getObjects := func() ([]*v1.Namespace, []*networkingv1.NetworkPolicy, []*v1.Pod) {
		namespace := rand.String(8)
		namespaces := []*v1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"app": namespace}},
			},
		}
		networkPolicies := []*networkingv1.NetworkPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "default-deny-all", UID: types.UID(uuid.New().String())},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "np-1", UID: types.UID(uuid.New().String())},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app-1": "scale-1"}},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{"app-1": "scale-1"},
									},
								},
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "np-2", UID: types.UID(uuid.New().String())},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app-2": "scale-2"}},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{"app-2": "scale-2"},
									},
								},
							},
						},
					},
				},
			},
		}
		pods := make([]*v1.Pod, 0, 50)
		podIdx := 0
		for i := 0; i < 25; i++ {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: fmt.Sprintf("pod-%d", podIdx), UID: types.UID(uuid.New().String()), Labels: map[string]string{"app-1": "scale-1"}},
				Spec:       v1.PodSpec{NodeName: getRandomNodeName()},
				Status:     v1.PodStatus{PodIP: getRandomIP()},
			}
			pods = append(pods, pod)
			podIdx++
		}
		for i := 0; i < 25; i++ {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: fmt.Sprintf("pod-%d", podIdx), UID: types.UID(uuid.New().String()), Labels: map[string]string{"app-2": "scale-2"}},
				Spec:       v1.PodSpec{NodeName: getRandomNodeName()},
				Status:     v1.PodStatus{PodIP: getRandomIP()},
			}
			pods = append(pods, pod)
			podIdx++
		}
		return namespaces, networkPolicies, pods
	}
	namespaces, nps, pods := getXObjects(5000, getObjects)

	return namespaces, pods, nps
}

func GenControllerTestInputs5() (
	[]*v1.Namespace,
	[]*v1.Pod,
	[]*networkingv1.NetworkPolicy,
) {
	getObjects := func() ([]*v1.Namespace, []*networkingv1.NetworkPolicy, []*v1.Pod) {
		namespace := rand.String(8)
		namespaces := []*v1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"app": namespace}},
			},
		}
		networkPolicies := []*networkingv1.NetworkPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "np-1", UID: types.UID(uuid.New().String())},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": namespace}},
								},
							},
						},
					},
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							To: []networkingv1.NetworkPolicyPeer{
								{
									NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": namespace}},
								},
							},
						},
					},
				},
			},
		}
		pods := make([]*v1.Pod, 0, 25)
		for i := 0; i < 25; i++ {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: fmt.Sprintf("pod-%d", i), UID: types.UID(uuid.New().String()), Labels: map[string]string{}},
				Spec:       v1.PodSpec{NodeName: getRandomNodeName()},
				Status:     v1.PodStatus{PodIP: getRandomIP()},
			}
			pods = append(pods, pod)
		}
		return namespaces, networkPolicies, pods
	}
	namespaces, nps, pods := getXObjects(1000, getObjects)

	return namespaces, pods, nps
}

func DumpMemStats(path string) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return ioutil.WriteFile(path, []byte(fmt.Sprintf("%+v\n", m)), 0644)
}

func MostRecentTime(times ...time.Time) time.Time {
	var m time.Time
	for _, t := range times {
		if t.After(m) {
			m = t
		}
	}
	return m
}

func getRandomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

func getRandomNodeName() string {
	return fmt.Sprintf("Node-%d", rand.Intn(1000))
}

// getXObjects calls the provided getObjectsFunc x times and aggregate the objects.
func getXObjects(x int, getObjectsFunc func() ([]*v1.Namespace, []*networkingv1.NetworkPolicy, []*v1.Pod)) ([]*v1.Namespace, []*networkingv1.NetworkPolicy, []*v1.Pod) {
	var namespaces []*v1.Namespace
	var networkPolicies []*networkingv1.NetworkPolicy
	var pods []*v1.Pod
	for i := 0; i < x; i++ {
		newNamespaces, newNetworkPolicies, newPods := getObjectsFunc()
		namespaces = append(namespaces, newNamespaces...)
		networkPolicies = append(networkPolicies, newNetworkPolicies...)
		pods = append(pods, newPods...)
	}
	return namespaces, networkPolicies, pods
}

func init() {
	rand.Seed(1234)
}
