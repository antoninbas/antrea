package ddlog

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
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

func testDataset(t *testing.T, objects ...runtime.Object) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := fake.NewSimpleClientset(objects...)
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

	mostRecentUpdate := func() time.Time {
		t1 := addressGroupStore.LastUpdate()
		t2 := appliedToGroupStore.LastUpdate()
		t3 := networkPolicyStore.LastUpdate()
		return ctesting.MostRecentTime(t1, t2, t3)
	}

	// ddlogProgram.StartRecordingCommands("/tmp/cmds.txt")

	start := time.Now()
	// Make sure informers are running.
	informerFactory.Start(ctx.Done())
	go c.Run(ctx.Done())
	for {
		now := time.Now()
		if now.Sub(mostRecentUpdate()) > 5*time.Second {
			break
		}
		fmt.Println("Waiting...")
		fmt.Printf(
			"AddressGroups: %v - AppliedToGroups: %v - NetworkPolicies: %v\n",
			len(addressGroupStore.List()), len(appliedToGroupStore.List()), len(networkPolicyStore.List()),
		)
		time.Sleep(time.Second)
	}
	end := mostRecentUpdate()
	fmt.Printf("Computation time: %v\n", end.Sub(start))
}

func TestPerf1(t *testing.T) {
	namespaces, pods, nps := ctesting.GenControllerTestInputs()
	objs := make([]runtime.Object, 0, len(namespaces)+len(pods)+len(nps))
	for i := range namespaces {
		objs = append(objs, namespaces[i])
	}
	for i := range pods {
		objs = append(objs, pods[i])
	}
	for i := range nps {
		objs = append(objs, nps[i])
	}
	testDataset(t, objs...)
}

func TestPerf2(t *testing.T) {
	namespaces, pods, nps := ctesting.GenControllerTestInputs2()
	objs := make([]runtime.Object, 0, len(namespaces)+len(pods)+len(nps))
	for i := range namespaces {
		objs = append(objs, namespaces[i])
	}
	for i := range nps {
		objs = append(objs, nps[i])
	}
	for i := range pods {
		objs = append(objs, pods[i])
	}
	testDataset(t, objs...)
}

func TestPerf3(t *testing.T) {
	namespaces, pods, nps := ctesting.GenControllerTestInputs3()
	objs := make([]runtime.Object, 0, len(namespaces)+len(pods)+len(nps))
	for i := range namespaces {
		objs = append(objs, namespaces[i])
	}
	for i := range nps {
		objs = append(objs, nps[i])
	}
	for i := range pods {
		objs = append(objs, pods[i])
	}
	testDataset(t, objs...)
}

func TestPerf4(t *testing.T) {
	namespaces, pods, nps := ctesting.GenControllerTestInputs4()
	objs := make([]runtime.Object, 0, len(namespaces)+len(pods)+len(nps))
	for i := range namespaces {
		objs = append(objs, namespaces[i])
	}
	for i := range nps {
		objs = append(objs, nps[i])
	}
	for i := range pods {
		objs = append(objs, pods[i])
	}
	testDataset(t, objs...)
}

func TestPerf5(t *testing.T) {
	namespaces, pods, nps := ctesting.GenControllerTestInputs5()
	objs := make([]runtime.Object, 0, len(namespaces)+len(pods)+len(nps))
	for i := range namespaces {
		objs = append(objs, namespaces[i])
	}
	for i := range nps {
		objs = append(objs, nps[i])
	}
	for i := range pods {
		objs = append(objs, pods[i])
	}
	testDataset(t, objs...)
}
