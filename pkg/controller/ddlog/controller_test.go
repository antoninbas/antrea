package ddlog

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/require"
	"github.com/vmware/differential-datalog/go/pkg/ddlog"

	"github.com/vmware-tanzu/antrea/pkg/controller/networkpolicy/store"
	ctesting "github.com/vmware-tanzu/antrea/pkg/controller/testing"
)

// https://github.com/kubernetes/client-go/blob/master/examples/fake-client/main_test.go

type outRecordHandler struct {
	controller *Controller
}

func (h *outRecordHandler) Handle(tableID ddlog.TableID, record ddlog.Record, polarity ddlog.OutPolarity) {
	h.controller.HandleRecordOut(tableID, record, polarity)
}

var alwaysReady = func() bool { return true }

func BenchmarkDDlogController(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// Create the fake client.
	client := fake.NewSimpleClientset()

	informerFactory := informers.NewSharedInformerFactory(client, 0)
	podInformer := informerFactory.Core().V1().Pods()
	namespaceInformer := informerFactory.Core().V1().Namespaces()
	networkPolicyInformer := informerFactory.Networking().V1().NetworkPolicies()

	handler := &outRecordHandler{
		controller: nil,
	}

	ddlogProgram, err := ddlog.NewProgram(1, handler)
	require.Nil(b, err)

	addressGroupStore := store.NewAddressGroupStore()
	appliedToGroupStore := store.NewAppliedToGroupStore()
	networkPolicyStore := store.NewNetworkPolicyStore()

	c := NewController(
		client,
		podInformer,
		namespaceInformer,
		networkPolicyInformer,
		ddlogProgram,
		addressGroupStore,
		appliedToGroupStore,
		networkPolicyStore,
	)
	handler.controller = c
	c.podListerSynced = alwaysReady
	c.namespaceListerSynced = alwaysReady
	c.networkPolicyListerSynced = alwaysReady

	// Make sure informers are running.
	informerFactory.Start(ctx.Done())

	go c.Run(ctx.Done())
	time.Sleep(3 * time.Second)

	namespaces, pods, nps := ctesting.GenControllerTestInputs(client)

	// ddlogProgram.StartRecordingCommands("/tmp/cmds.txt")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctesting.CreateControllerTestInputs(b, client, namespaces, pods, nps)

		for len(networkPolicyStore.List()) != len(nps) {
			time.Sleep(10 * time.Millisecond)
		}

		ctesting.DeleteControllerTestInputs(b, client, namespaces, pods, nps)

		for len(networkPolicyStore.List()) != 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	b.StopTimer()

	time.Sleep(1 * time.Second)

	cancel()

	time.Sleep(3 * time.Second)
}

func TestDDlogController(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// Create the fake client.
	client := fake.NewSimpleClientset()

	informerFactory := informers.NewSharedInformerFactory(client, 0)
	podInformer := informerFactory.Core().V1().Pods()
	namespaceInformer := informerFactory.Core().V1().Namespaces()
	networkPolicyInformer := informerFactory.Networking().V1().NetworkPolicies()

	handler := &outRecordHandler{
		controller: nil,
	}

	ddlogProgram, err := ddlog.NewProgram(1, handler)
	require.Nil(t, err)

	addressGroupStore := store.NewAddressGroupStore()
	appliedToGroupStore := store.NewAppliedToGroupStore()
	networkPolicyStore := store.NewNetworkPolicyStore()

	c := NewController(
		client,
		podInformer,
		namespaceInformer,
		networkPolicyInformer,
		ddlogProgram,
		addressGroupStore,
		appliedToGroupStore,
		networkPolicyStore,
	)
	handler.controller = c
	c.podListerSynced = alwaysReady
	c.namespaceListerSynced = alwaysReady
	c.networkPolicyListerSynced = alwaysReady

	// Make sure informers are running.
	informerFactory.Start(ctx.Done())

	go c.Run(ctx.Done())
	time.Sleep(3 * time.Second)

	namespaces, pods, nps := ctesting.GenControllerTestInputs(client)

	// ddlogProgram.StartRecordingCommands("/tmp/cmds.txt")

	for i := 0; i < 10; i++ {
		fmt.Printf("Loop %v\n", i)
		ctesting.CreateControllerTestInputs(t, client, namespaces, pods, nps)

		for len(networkPolicyStore.List()) != len(nps) {
			time.Sleep(10 * time.Millisecond)
		}

		ctesting.DeleteControllerTestInputs(t, client, namespaces, pods, nps)

		for len(networkPolicyStore.List()) != 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	time.Sleep(1 * time.Second)

	cancel()

	time.Sleep(3 * time.Second)

	ddlogProgram.Stop()
}
