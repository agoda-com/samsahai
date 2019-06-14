package desiredcomponent

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/apis/env/v1beta1"
	envv1beta1 "github.com/agoda-com/samsahai/internal/apis/env/v1beta1"
	samsahaiconfig "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/desiredcomponent"
	"github.com/agoda-com/samsahai/internal/queue"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "desired component controller")
}

var _ = Describe("desired component [e2e]", func() {
	var queueCtrl internal.QueueController
	var desiredComponentCtrl internal.DesiredComponentController
	var namespace string
	var wg = sync.WaitGroup{}
	stop := make(chan struct{})

	GinkgoRecover()

	BeforeSuite(func() {
		Expect(os.Getenv("POD_NAMESPACE")).NotTo(BeEmpty(), "POD_NAMESPACE should be provided")

		namespace = os.Getenv("POD_NAMESPACE")

		cfg, err := config.GetConfig()
		Expect(err).To(BeNil(), "Please provide credential for accessing k8s cluster")

		mgr, err := manager.New(cfg, manager.Options{Namespace: namespace})
		Expect(err).To(BeNil(), "should create manager successfully")

		err = envv1beta1.AddToScheme(mgr.GetScheme())
		Expect(err).To(BeNil(), "should register scheme successfully")

		restClient, err := rest.UnversionedRESTClientFor(samsahaiconfig.GetRESTConfg(cfg, &v1beta1.SchemeGroupVersion))
		Expect(err).To(BeNil(), "should create rest client successfully")

		queueCtrl = queue.NewWithClient(namespace, restClient)
		Expect(queueCtrl).NotTo(BeNil(), "should create queue ctrl successfully")
		desiredComponentCtrl = desiredcomponent.NewWithClient(namespace, mgr, restClient, queueCtrl)
		Expect(desiredComponentCtrl).NotTo(BeNil(), "should create desired component ctrl successfully")

		go func() {
			err := mgr.Start(stop)
			Expect(err).To(BeNil())
			wg.Done()
		}()
		wg.Add(1)
	}, 10)

	AfterSuite(func() {
		desiredComponentCtrl.Clear()
		close(stop)
		wg.Wait()
	}, 5)

	BeforeEach(func() {
		err := queueCtrl.RemoveAll()
		Expect(err).To(BeNil())
		desiredComponentCtrl.Clear()
		desiredComponentCtrl.Start()
	}, 5)

	AfterEach(func() {
		desiredComponentCtrl.Stop()
		fmt.Println("after each done!")
	}, 5)

	It("should successfully ...", func() {
		desiredComponentCtrl.TryCheck("alpine")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done := make(chan struct{})

		go func() {
			for {
				if queueCtrl.Size() == 1 {
					done <- struct{}{}
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
		}()

		select {
		case <-ctx.Done():
			Expect(false).To(BeTrue(), "timeout")
		case <-done:
			Expect(true).To(BeTrue())
		}

		err := queueCtrl.RemoveAll()
		Expect(err).To(BeNil())
	})

})
