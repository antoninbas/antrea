// Copyright 2022 Antrea Authors
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

package podwatch

import (
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	netdefclientfake "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"

	podwatchtesting "antrea.io/antrea/pkg/agent/secondarynetwork/podwatch/testing"
)

func TestPodController(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, resyncPeriod)
	podCache := cnipodcache.NewCNIPodInfoStore()
	interfaceConfigurator := podwatchtesting.NewMockInterfaceConfigurator(ctrl)
	podController := &PodController{
		kubeClient:            client,
		netAttachDefClient:    netdefclientfake.NewSimpleClientset(),
		queue:                 workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "podcontroller"),
		podInformer:           informerFactory.Core().V1().Pods(),
		nodeName:              "test-node",
		podCache:              podCache,
		interfaceConfigurator: interfaceConfigurator,
	}

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		podController.Run(stopCh)
	}()

	close(stopCh)
	wg.Wait()
}
