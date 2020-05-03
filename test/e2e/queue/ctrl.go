package queue

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/queue"
)

var _ = Describe("[e2e] Queue controller", func() {
	var controller internal.QueueController
	var namespace string
	var client rclient.Client
	var teamName = "example"

	BeforeEach(func(done Done) {
		defer close(done)

		namespace = os.Getenv("POD_NAMESPACE")
		Expect(namespace).NotTo(BeEmpty(), "Please provided POD_NAMESPACE")

		cfg, err := config.GetConfig()
		Expect(err).To(BeNil(), "Please provide credential for accessing k8s cluster")

		client, err = rclient.New(cfg, rclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())

		controller = queue.New(namespace, client)
		Expect(controller).NotTo(BeNil(), "Should successfully init Queue controller")

		Expect(controller.RemoveAllQueues()).To(BeNil())
	}, 5)

	AfterEach(func(done Done) {
		defer close(done)

		Expect(controller.RemoveAllQueues()).To(BeNil())
	}, 5)

	It("should successfully create/get/delete Queue", func(done Done) {
		defer close(done)

		q := queue.NewUpgradeQueue(teamName, namespace, "alpine", "alpine", "3.9.3")

		var err = controller.Add(q)
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
	}, 3)

	It("should successfully remove old Queue", func(done Done) {
		defer close(done)

		var err error
		var first *v1beta1.Queue
		var alpine390 = queue.NewUpgradeQueue(teamName, namespace, "alpine", "alpine", "3.9.0")
		var alpine391 = queue.NewUpgradeQueue(teamName, namespace, "alpine", "alpine", "3.9.1")
		var alpine392 = queue.NewUpgradeQueue(teamName, namespace, "alpine", "alpine", "3.9.2")
		var ubuntu = queue.NewUpgradeQueue(teamName, namespace, "ubuntu", "ubuntu", "18.04")
		var size int

		Expect(controller.Size()).To(Equal(0), "should start with empty queue")

		By("Create 3 queue")

		err = controller.Add(alpine390)
		Expect(err).To(BeNil())
		err = controller.Add(ubuntu)
		Expect(err).To(BeNil())
		err = controller.Add(queue.NewUpgradeQueue(teamName, namespace, "node", "node", "11.0.0"))
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

		err = controller.RemoveAllQueues()
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(0))
	}, 3)
})
