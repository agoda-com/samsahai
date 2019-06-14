package queue

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/apis/env/v1beta1"
	"github.com/agoda-com/samsahai/internal/queue"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "queue controller")
}

var _ = Describe("queue controller [e2e]", func() {
	var controller internal.QueueController
	var namespace string

	GinkgoRecover()

	BeforeSuite(func() {
		Expect(os.Getenv("POD_NAMESPACE")).NotTo(BeEmpty(), "POD_NAMESPACE should be provided")

		namespace = os.Getenv("POD_NAMESPACE")

		cfg, err := config.GetConfig()
		Expect(err).To(BeNil(), "Please provide credential for accessing k8s cluster")

		controller = queue.New(namespace, cfg)
		Expect(controller).NotTo(BeNil())
	})

	BeforeEach(func() {
		err := controller.RemoveAll()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {

	})

	It("should successfully create/get/delete Queue", func() {
		q := queue.NewUpgradeQueue(namespace, "alpine", "alpine", "3.9.3")
		var err error

		err = controller.Add(q)
		Expect(err).To(BeNil())

		size := controller.Size()
		Expect(size).To(Equal(1))

		first, err := controller.First()
		Expect(err).To(BeNil())
		Expect(first.IsSame(q)).To(BeTrue())

		err = controller.Remove(first)
		Expect(err).To(BeNil())

		size = controller.Size()
		Expect(size).To(Equal(0))
	})

	It("should successfully remove old Queue", func() {
		var err error
		var first *v1beta1.Queue
		var alpine390 = queue.NewUpgradeQueue(namespace, "alpine", "alpine", "3.9.0")
		var alpine391 = queue.NewUpgradeQueue(namespace, "alpine", "alpine", "3.9.1")
		var alpine392 = queue.NewUpgradeQueue(namespace, "alpine", "alpine", "3.9.2")
		var ubuntu = queue.NewUpgradeQueue(namespace, "ubuntu", "ubuntu", "18.04")
		var size int

		Expect(controller.Size()).To(Equal(0), "should start with empty queue")

		By("Create 3 queue")

		err = controller.Add(alpine390)
		Expect(err).To(BeNil())
		err = controller.Add(ubuntu)
		Expect(err).To(BeNil())
		err = controller.Add(queue.NewUpgradeQueue(namespace, "node", "node", "11.0.0"))
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(3))

		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(first.IsSame(alpine390)).To(BeTrue())

		By("Adding alpine 3.9.1")

		err = controller.Add(alpine391)
		Expect(err).To(BeNil())
		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(first.IsSame(ubuntu)).To(BeTrue(), "ubuntu should be on top of queue")
		size = controller.Size()
		Expect(size).To(Equal(3), "size of queue should remain 3")

		By("Adding alpine 3.9.2 at top queue")

		err = controller.AddTop(alpine392)
		Expect(err).To(BeNil())
		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(first.IsSame(alpine392)).To(BeTrue(), "alpine 3.9.2 should be on top of queue")
		size = controller.Size()
		Expect(size).To(Equal(3), "size of queue should remain 3")

		By("Re-order ubuntu to top queue")

		err = controller.AddTop(ubuntu)
		Expect(err).To(BeNil())
		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(first.IsSame(ubuntu)).To(BeTrue(), "ubuntu should be on top of queue")

		By("Removing all queues")

		err = controller.RemoveAll()
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(0))
	})
})
