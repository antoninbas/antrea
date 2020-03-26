package testing

import (
	"fmt"
	"io/ioutil"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func GenControllerTestInputs(client *fake.Clientset) (
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

func CreateControllerTestInputs(
	t testing.TB,
	client *fake.Clientset,
	namespaces []*v1.Namespace,
	pods []*v1.Pod,
	nps []*networkingv1.NetworkPolicy,
) {
	for _, ns := range namespaces {
		_, err := client.CoreV1().Namespaces().Create(ns)
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	for _, pod := range pods {
		_, err := client.CoreV1().Pods(pod.Namespace).Create(pod)
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	for _, np := range nps {
		_, err := client.NetworkingV1().NetworkPolicies(np.Namespace).Create(np)
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}
}

func DeleteControllerTestInputs(
	t testing.TB,
	client *fake.Clientset,
	namespaces []*v1.Namespace,
	pods []*v1.Pod,
	nps []*networkingv1.NetworkPolicy,
) {
	for _, np := range nps {
		err := client.NetworkingV1().NetworkPolicies(np.Namespace).Delete(np.Name, &metav1.DeleteOptions{})
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	for _, pod := range pods {
		err := client.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	for _, ns := range namespaces {
		err := client.CoreV1().Namespaces().Delete(ns.Name, &metav1.DeleteOptions{})
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	client.ClearActions()
}

func GenControllerTestInputs2(client *fake.Clientset) (
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

func CreateControllerTestInputs2(
	t testing.TB,
	client *fake.Clientset,
	namespaces []*v1.Namespace,
	pods []*v1.Pod,
	nps []*networkingv1.NetworkPolicy,
) {
	for _, ns := range namespaces {
		_, err := client.CoreV1().Namespaces().Create(ns)
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	for _, np := range nps {
		_, err := client.NetworkingV1().NetworkPolicies(np.Namespace).Create(np)
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	for _, pod := range pods {
		_, err := client.CoreV1().Pods(pod.Namespace).Create(pod)
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}
}

func DeleteControllerTestInputs2(
	t testing.TB,
	client *fake.Clientset,
	namespaces []*v1.Namespace,
	pods []*v1.Pod,
	nps []*networkingv1.NetworkPolicy,
) {
	for _, pod := range pods {
		err := client.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	for _, np := range nps {
		err := client.NetworkingV1().NetworkPolicies(np.Namespace).Delete(np.Name, &metav1.DeleteOptions{})
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	for _, ns := range namespaces {
		err := client.CoreV1().Namespaces().Delete(ns.Name, &metav1.DeleteOptions{})
		require.Nil(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	client.ClearActions()
}

func DumpMemStats(path string) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return ioutil.WriteFile(path, []byte(fmt.Sprintf("%+v\n", m)), 0644)
}
