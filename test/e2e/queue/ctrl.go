package queue

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/queue"
)

var _ = Describe("[e2e] Queue controller", func() {
	var controller internal.QueueController
	var namespace string
	var client rclient.Client
	var teamName = "teamtest-queue"

	BeforeEach(func() {

		namespace = os.Getenv("POD_NAMESPACE")
		Expect(namespace).NotTo(BeEmpty(), "Please provide POD_NAMESPACE")

		cfg, err := config.GetConfig()
		Expect(err).To(BeNil(), "Please provide credential for accessing k8s cluster")

		client, err = rclient.New(cfg, rclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())

		controller = queue.New(namespace, client)
		Expect(controller).NotTo(BeNil(), "Should successfully init Queue controller")

		Expect(controller.RemoveAllQueues(namespace)).To(BeNil())
	}, 5)

	AfterEach(func() {

		Expect(controller.RemoveAllQueues(namespace)).To(BeNil())
	}, 5)

	It("should successfully create/get/delete Queue", func() {
		q := queue.NewQueue(teamName, namespace, "alpine", "",
			s2hv1.QueueComponents{{Name: "alpine", Version: "3.9.3"}}, s2hv1.QueueTypeUpgrade,
		)

		var err = controller.Add(q, nil)
		Expect(err).To(BeNil())

		size := controller.Size(namespace)
		Expect(size).To(Equal(1))

		first, err := controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(first.(*s2hv1.Queue).ContainSameComponent(q.Name, q.Spec.Components[0])).To(BeTrue())

		err = controller.Remove(first)
		Expect(err).To(BeNil())

		size = controller.Size(namespace)
		Expect(size).To(Equal(0))
	}, 3)

	It("should successfully remove old Queue", func() {
		var err error
		var first runtime.Object
		var alpine390 = queue.NewQueue(teamName, namespace, "alpine", "",
			s2hv1.QueueComponents{{Name: "alpine", Version: "3.9.0"}}, s2hv1.QueueTypeUpgrade,
		)
		var alpine391 = queue.NewQueue(teamName, namespace, "alpine", "",
			s2hv1.QueueComponents{{Name: "alpine", Version: "3.9.1"}}, s2hv1.QueueTypeUpgrade,
		)
		var alpine392 = queue.NewQueue(teamName, namespace, "alpine", "",
			s2hv1.QueueComponents{{Name: "alpine", Version: "3.9.2"}}, s2hv1.QueueTypeUpgrade,
		)
		var ubuntu = queue.NewQueue(teamName, namespace, "ubuntu", "",
			s2hv1.QueueComponents{{Name: "ubuntu", Version: "18.04"}}, s2hv1.QueueTypeUpgrade,
		)
		var size int

		Expect(controller.Size(namespace)).To(Equal(0), "should start with empty queue")

		By("Create 3 queues")

		err = controller.Add(alpine390, nil)
		Expect(err).To(BeNil())
		err = controller.Add(ubuntu, nil)
		Expect(err).To(BeNil())
		err = controller.Add(queue.NewQueue(teamName, namespace, "node", "",
			s2hv1.QueueComponents{{Name: "node", Version: "11.0.0"}}, s2hv1.QueueTypeUpgrade,
		), nil)
		Expect(err).To(BeNil())
		size = controller.Size(namespace)
		Expect(size).To(Equal(3))

		first, err = controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(first.(*s2hv1.Queue).ContainSameComponent(alpine390.Name, alpine390.Spec.Components[0])).To(BeTrue())

		By("Adding alpine 3.9.1")

		err = controller.Add(alpine391, nil)
		Expect(err).To(BeNil())
		first, err = controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(first.(*s2hv1.Queue).ContainSameComponent(ubuntu.Name, ubuntu.Spec.Components[0])).To(BeTrue(),
			"ubuntu should be on top of queue")
		size = controller.Size(namespace)
		Expect(size).To(Equal(3), "size of queue should remain 3")

		By("Adding alpine 3.9.2 at top queue")

		err = controller.AddTop(alpine392)
		Expect(err).To(BeNil())
		first, err = controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(first.(*s2hv1.Queue).ContainSameComponent(alpine392.Name, alpine392.Spec.Components[0])).To(BeTrue(),
			"alpine 3.9.2 should be on top of queue")
		size = controller.Size(namespace)
		Expect(size).To(Equal(3), "size of queue should remain 3")

		By("Re-order ubuntu to top queue")

		err = controller.AddTop(ubuntu)
		Expect(err).To(BeNil())
		first, err = controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(first.(*s2hv1.Queue).ContainSameComponent(ubuntu.Name, ubuntu.Spec.Components[0])).To(BeTrue(),
			"ubuntu should be on top of queue")

		By("Removing all queues")

		err = controller.RemoveAllQueues(namespace)
		Expect(err).To(BeNil())
		size = controller.Size(namespace)
		Expect(size).To(Equal(0))
	}, 3)

	It("should successfully add component to existing bundle", func() {
		var err error
		var first runtime.Object
		var alpine = queue.NewQueue(teamName, namespace, "group", "group",
			s2hv1.QueueComponents{{Name: "alpine", Version: "3.9.0"}}, s2hv1.QueueTypeUpgrade,
		)
		var ubuntu = queue.NewQueue(teamName, namespace, "ubuntu", "",
			s2hv1.QueueComponents{{Name: "ubuntu", Version: "18.04"}}, s2hv1.QueueTypeUpgrade,
		)
		var node = queue.NewQueue(teamName, namespace, "group", "group",
			s2hv1.QueueComponents{{Name: "node", Version: "11.0.0"}}, s2hv1.QueueTypeUpgrade,
		)
		var size int

		Expect(controller.Size(namespace)).To(Equal(0), "should start with empty queue")

		By("Create 2 queues")

		err = controller.Add(alpine, nil)
		Expect(err).To(BeNil())
		err = controller.Add(ubuntu, nil)
		Expect(err).To(BeNil())

		By("Add node application")

		err = controller.Add(node, nil)
		Expect(err).To(BeNil())
		size = controller.Size(namespace)
		Expect(size).To(Equal(2))

		first, err = controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(len(first.(*s2hv1.Queue).Spec.Components)).To(Equal(2))
		Expect(first.(*s2hv1.Queue).ContainSameComponent("group", alpine.Spec.Components[0])).To(BeTrue())
		Expect(first.(*s2hv1.Queue).ContainSameComponent("group", node.Spec.Components[0])).To(BeTrue())

		By("Removing all queues")

		err = controller.RemoveAllQueues(namespace)
		Expect(err).To(BeNil())
		size = controller.Size(namespace)
		Expect(size).To(Equal(0))
	}, 3)

	It("should successfully update component in existing bundle", func() {
		var err error
		var first runtime.Object
		var application = queue.NewQueue(teamName, namespace, "group", "group",
			s2hv1.QueueComponents{
				{Name: "alpine", Version: "3.9.0"},
				{Name: "node", Version: "11.0.0"},
			}, s2hv1.QueueTypeUpgrade,
		)
		var ubuntu = queue.NewQueue(teamName, namespace, "ubuntu", "",
			s2hv1.QueueComponents{{Name: "ubuntu", Version: "18.04"}}, s2hv1.QueueTypeUpgrade,
		)
		var node = queue.NewQueue(teamName, namespace, "group", "group",
			s2hv1.QueueComponents{{Name: "node", Version: "11.0.2"}}, s2hv1.QueueTypeUpgrade,
		)
		var size int

		Expect(controller.Size(namespace)).To(Equal(0), "should start with empty queue")

		By("Create 2 queues")

		err = controller.Add(application, nil)
		Expect(err).To(BeNil())
		err = controller.Add(ubuntu, nil)
		Expect(err).To(BeNil())

		By("Update node application")

		err = controller.Add(node, nil)
		Expect(err).To(BeNil())
		size = controller.Size(namespace)
		Expect(size).To(Equal(2))

		first, err = controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(len(first.(*s2hv1.Queue).Spec.Components)).To(Equal(2))
		Expect(first.(*s2hv1.Queue).ContainSameComponent("group", application.Spec.Components[0])).To(BeTrue())
		Expect(first.(*s2hv1.Queue).ContainSameComponent("group", node.Spec.Components[0])).To(BeTrue())

		By("Removing all queues")

		err = controller.RemoveAllQueues(namespace)
		Expect(err).To(BeNil())
		size = controller.Size(namespace)
		Expect(size).To(Equal(0))
	}, 3)

	It("should successfully remove queue/component from existing bundle and create new queue", func() {
		var err error
		var first runtime.Object
		var application = queue.NewQueue(teamName, namespace, "group", "group",
			s2hv1.QueueComponents{
				{Name: "alpine", Version: "3.9.0"},
				{Name: "node", Version: "11.0.0"},
			}, s2hv1.QueueTypeUpgrade,
		)
		var duplicatedNode = queue.NewQueue(teamName, namespace, "node", "",
			s2hv1.QueueComponents{{Name: "node", Version: "11.0.1"}}, s2hv1.QueueTypeUpgrade,
		)
		var node = queue.NewQueue(teamName, namespace, "node", "",
			s2hv1.QueueComponents{{Name: "node", Version: "11.0.2"}}, s2hv1.QueueTypeUpgrade,
		)
		var size int

		Expect(controller.Size(namespace)).To(Equal(0), "should start with empty queue")

		By("Create 2 queues")

		err = controller.Add(application, nil)
		Expect(err).To(BeNil())
		err = controller.Add(duplicatedNode, nil)
		Expect(err).To(BeNil())

		By("Update node application to not in bundle")

		err = controller.Add(node, nil)
		Expect(err).To(BeNil())
		size = controller.Size(namespace)
		Expect(size).To(Equal(2))

		first, err = controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(len(first.(*s2hv1.Queue).Spec.Components)).To(Equal(1))
		Expect(first.(*s2hv1.Queue).ContainSameComponent("group", application.Spec.Components[0])).To(BeTrue())

		By("Removing all queues")

		err = controller.RemoveAllQueues(namespace)
		Expect(err).To(BeNil())
		size = controller.Size(namespace)
		Expect(size).To(Equal(0))
	}, 3)

	It("should successfully create queue following priority queues", func() {
		priorityQueues := []string{"alpine", "ubuntu"}

		var err error
		var size int
		var alpine = queue.NewQueue(teamName, namespace, "group", "group",
			s2hv1.QueueComponents{{Name: "alpine", Version: "3.9.1"}}, s2hv1.QueueTypeUpgrade,
		)
		var ubuntu16 = queue.NewQueue(teamName, namespace, "ubuntu", "",
			s2hv1.QueueComponents{{Name: "ubuntu", Version: "16.04"}}, s2hv1.QueueTypeUpgrade,
		)
		var ubuntu18 = queue.NewQueue(teamName, namespace, "ubuntu", "",
			s2hv1.QueueComponents{{Name: "ubuntu", Version: "18.04"}}, s2hv1.QueueTypeUpgrade,
		)
		var node = queue.NewQueue(teamName, namespace, "group", "group",
			s2hv1.QueueComponents{{Name: "node", Version: "11.0.0"}}, s2hv1.QueueTypeUpgrade,
		)

		Expect(controller.Size(namespace)).To(Equal(0), "should start with empty queue")

		By("Create 1 bundle queue")

		err = controller.Add(node, priorityQueues)
		Expect(err).To(BeNil())

		By("Create 1 queue with higher priority")

		err = controller.Add(ubuntu16, priorityQueues)
		Expect(err).To(BeNil())

		size = controller.Size(namespace)
		Expect(size).To(Equal(2))

		first, err := controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(first.(*s2hv1.Queue).ContainSameComponent(ubuntu16.Name, ubuntu16.Spec.Components[0])).To(BeTrue())

		By("Create 1 bundle queue with the highest priority")

		err = controller.Add(alpine, priorityQueues)
		Expect(err).To(BeNil())

		size = controller.Size(namespace)
		Expect(size).To(Equal(2))

		first, err = controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(first.(*s2hv1.Queue).ContainSameComponent(alpine.Name, alpine.Spec.Components[0])).To(BeTrue())
		Expect(first.(*s2hv1.Queue).ContainSameComponent(node.Name, node.Spec.Components[0])).To(BeTrue())

		By("Create 1 queue with lower priority")

		err = controller.Add(ubuntu18, priorityQueues)
		Expect(err).To(BeNil())

		size = controller.Size(namespace)
		Expect(size).To(Equal(2))

		first, err = controller.First(namespace)
		Expect(err).To(BeNil())
		Expect(first.(*s2hv1.Queue).ContainSameComponent(alpine.Name, alpine.Spec.Components[0])).To(BeTrue())
		Expect(first.(*s2hv1.Queue).ContainSameComponent(node.Name, node.Spec.Components[0])).To(BeTrue())

		By("Removing all queues")

		err = controller.RemoveAllQueues(namespace)
		Expect(err).To(BeNil())
		size = controller.Size(namespace)
		Expect(size).To(Equal(0))
	}, 3)
})
