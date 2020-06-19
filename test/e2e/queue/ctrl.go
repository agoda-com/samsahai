package queue

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
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

		q := queue.NewUpgradeQueue(teamName, namespace, "alpine", "",
			s2hv1beta1.QueueComponents{{Name: "alpine", Version: "3.9.3"}},
		)

		var err = controller.Add(q, nil)
		Expect(err).To(BeNil())

		size := controller.Size()
		Expect(size).To(Equal(1))

		first, err := controller.First()
		Expect(err).To(BeNil())
		Expect(first.ContainSameComponent(q.Name, q.Spec.Components[0])).To(BeTrue())

		err = controller.Remove(first)
		Expect(err).To(BeNil())

		size = controller.Size()
		Expect(size).To(Equal(0))
	}, 3)

	It("should successfully remove old Queue", func(done Done) {
		defer close(done)

		var err error
		var first *s2hv1beta1.Queue
		var alpine390 = queue.NewUpgradeQueue(teamName, namespace, "alpine", "",
			s2hv1beta1.QueueComponents{{Name: "alpine", Version: "3.9.0"}},
		)
		var alpine391 = queue.NewUpgradeQueue(teamName, namespace, "alpine", "",
			s2hv1beta1.QueueComponents{{Name: "alpine", Version: "3.9.1"}},
		)
		var alpine392 = queue.NewUpgradeQueue(teamName, namespace, "alpine", "",
			s2hv1beta1.QueueComponents{{Name: "alpine", Version: "3.9.2"}},
		)
		var ubuntu = queue.NewUpgradeQueue(teamName, namespace, "ubuntu", "",
			s2hv1beta1.QueueComponents{{Name: "ubuntu", Version: "18.04"}},
		)
		var size int

		Expect(controller.Size()).To(Equal(0), "should start with empty queue")

		By("Create 3 queues")

		err = controller.Add(alpine390, nil)
		Expect(err).To(BeNil())
		err = controller.Add(ubuntu, nil)
		Expect(err).To(BeNil())
		err = controller.Add(queue.NewUpgradeQueue(teamName, namespace, "node", "",
			s2hv1beta1.QueueComponents{{Name: "node", Version: "11.0.0"}},
		), nil)
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(3))

		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(first.ContainSameComponent(alpine390.Name, alpine390.Spec.Components[0])).To(BeTrue())

		By("Adding alpine 3.9.1")

		err = controller.Add(alpine391, nil)
		Expect(err).To(BeNil())
		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(first.ContainSameComponent(ubuntu.Name, ubuntu.Spec.Components[0])).To(BeTrue(), "ubuntu should be on top of queue")
		size = controller.Size()
		Expect(size).To(Equal(3), "size of queue should remain 3")

		By("Adding alpine 3.9.2 at top queue")

		err = controller.AddTop(alpine392)
		Expect(err).To(BeNil())
		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(first.ContainSameComponent(alpine392.Name, alpine392.Spec.Components[0])).To(BeTrue(), "alpine 3.9.2 should be on top of queue")
		size = controller.Size()
		Expect(size).To(Equal(3), "size of queue should remain 3")

		By("Re-order ubuntu to top queue")

		err = controller.AddTop(ubuntu)
		Expect(err).To(BeNil())
		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(first.ContainSameComponent(ubuntu.Name, ubuntu.Spec.Components[0])).To(BeTrue(), "ubuntu should be on top of queue")

		By("Removing all queues")

		err = controller.RemoveAllQueues()
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(0))
	}, 3)

	It("should successfully add component to existing bundle", func(done Done) {
		defer close(done)

		var err error
		var first *s2hv1beta1.Queue
		var alpine = queue.NewUpgradeQueue(teamName, namespace, "group", "group",
			s2hv1beta1.QueueComponents{{Name: "alpine", Version: "3.9.0"}},
		)
		var ubuntu = queue.NewUpgradeQueue(teamName, namespace, "ubuntu", "",
			s2hv1beta1.QueueComponents{{Name: "ubuntu", Version: "18.04"}},
		)
		var node = queue.NewUpgradeQueue(teamName, namespace, "group", "group",
			s2hv1beta1.QueueComponents{{Name: "node", Version: "11.0.0"}},
		)
		var size int

		Expect(controller.Size()).To(Equal(0), "should start with empty queue")

		By("Create 2 queues")

		err = controller.Add(alpine, nil)
		Expect(err).To(BeNil())
		err = controller.Add(ubuntu, nil)
		Expect(err).To(BeNil())

		By("Add node application")

		err = controller.Add(node, nil)
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(2))

		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(len(first.Spec.Components)).To(Equal(2))
		Expect(first.ContainSameComponent("group", alpine.Spec.Components[0])).To(BeTrue())
		Expect(first.ContainSameComponent("group", node.Spec.Components[0])).To(BeTrue())

		By("Removing all queues")

		err = controller.RemoveAllQueues()
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(0))
	}, 3)

	It("should successfully update component in existing bundle", func(done Done) {
		defer close(done)

		var err error
		var first *s2hv1beta1.Queue
		var application = queue.NewUpgradeQueue(teamName, namespace, "group", "group",
			s2hv1beta1.QueueComponents{
				{Name: "alpine", Version: "3.9.0"},
				{Name: "node", Version: "11.0.0"},
			},
		)
		var ubuntu = queue.NewUpgradeQueue(teamName, namespace, "ubuntu", "",
			s2hv1beta1.QueueComponents{{Name: "ubuntu", Version: "18.04"}},
		)
		var node = queue.NewUpgradeQueue(teamName, namespace, "group", "group",
			s2hv1beta1.QueueComponents{{Name: "node", Version: "11.0.2"}},
		)
		var size int

		Expect(controller.Size()).To(Equal(0), "should start with empty queue")

		By("Create 2 queues")

		err = controller.Add(application, nil)
		Expect(err).To(BeNil())
		err = controller.Add(ubuntu, nil)
		Expect(err).To(BeNil())

		By("Update node application")

		err = controller.Add(node, nil)
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(2))

		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(len(first.Spec.Components)).To(Equal(2))
		Expect(first.ContainSameComponent("group", application.Spec.Components[0])).To(BeTrue())
		Expect(first.ContainSameComponent("group", node.Spec.Components[0])).To(BeTrue())

		By("Removing all queues")

		err = controller.RemoveAllQueues()
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(0))
	}, 3)

	It("should successfully remove queue/component from existing bundle and create new queue", func(done Done) {
		defer close(done)

		var err error
		var first *s2hv1beta1.Queue
		var application = queue.NewUpgradeQueue(teamName, namespace, "group", "group",
			s2hv1beta1.QueueComponents{
				{Name: "alpine", Version: "3.9.0"},
				{Name: "node", Version: "11.0.0"},
			},
		)
		var duplicatedNode = queue.NewUpgradeQueue(teamName, namespace, "node", "",
			s2hv1beta1.QueueComponents{{Name: "node", Version: "11.0.1"}},
		)
		var node = queue.NewUpgradeQueue(teamName, namespace, "node", "",
			s2hv1beta1.QueueComponents{{Name: "node", Version: "11.0.2"}},
		)
		var size int

		Expect(controller.Size()).To(Equal(0), "should start with empty queue")

		By("Create 2 queues")

		err = controller.Add(application, nil)
		Expect(err).To(BeNil())
		err = controller.Add(duplicatedNode, nil)
		Expect(err).To(BeNil())

		By("Update node application to not in bundle")

		err = controller.Add(node, nil)
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(2))

		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(len(first.Spec.Components)).To(Equal(1))
		Expect(first.ContainSameComponent("group", application.Spec.Components[0])).To(BeTrue())

		By("Removing all queues")

		err = controller.RemoveAllQueues()
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(0))
	}, 3)

	It("should successfully create queue following priority queues", func(done Done) {
		defer close(done)

		priorityQueues := []string{"alpine", "ubuntu"}

		var err error
		var size int
		var alpine = queue.NewUpgradeQueue(teamName, namespace, "group", "group",
			s2hv1beta1.QueueComponents{{Name: "alpine", Version: "3.9.1"}},
		)
		var ubuntu16 = queue.NewUpgradeQueue(teamName, namespace, "ubuntu", "",
			s2hv1beta1.QueueComponents{{Name: "ubuntu", Version: "16.04"}},
		)
		var ubuntu18 = queue.NewUpgradeQueue(teamName, namespace, "ubuntu", "",
			s2hv1beta1.QueueComponents{{Name: "ubuntu", Version: "18.04"}},
		)
		var node = queue.NewUpgradeQueue(teamName, namespace, "group", "group",
			s2hv1beta1.QueueComponents{{Name: "node", Version: "11.0.0"}},
		)

		Expect(controller.Size()).To(Equal(0), "should start with empty queue")

		By("Create 1 bundle queue")

		err = controller.Add(node, priorityQueues)
		Expect(err).To(BeNil())

		By("Create 1 queue with higher priority")

		err = controller.Add(ubuntu16, priorityQueues)
		Expect(err).To(BeNil())

		size = controller.Size()
		Expect(size).To(Equal(2))

		first, err := controller.First()
		Expect(err).To(BeNil())
		Expect(first.ContainSameComponent(ubuntu16.Name, ubuntu16.Spec.Components[0])).To(BeTrue())

		By("Create 1 bundle queue with the highest priority")

		err = controller.Add(alpine, priorityQueues)
		Expect(err).To(BeNil())

		size = controller.Size()
		Expect(size).To(Equal(2))

		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(first.ContainSameComponent(alpine.Name, alpine.Spec.Components[0])).To(BeTrue())
		Expect(first.ContainSameComponent(node.Name, node.Spec.Components[0])).To(BeTrue())

		By("Create 1 queue with lower priority")

		err = controller.Add(ubuntu18, priorityQueues)
		Expect(err).To(BeNil())

		size = controller.Size()
		Expect(size).To(Equal(2))

		first, err = controller.First()
		Expect(err).To(BeNil())
		Expect(first.ContainSameComponent(alpine.Name, alpine.Spec.Components[0])).To(BeTrue())
		Expect(first.ContainSameComponent(node.Name, node.Spec.Components[0])).To(BeTrue())

		By("Removing all queues")

		err = controller.RemoveAllQueues()
		Expect(err).To(BeNil())
		size = controller.Size()
		Expect(size).To(Equal(0))
	}, 3)
})
