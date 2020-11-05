package stablecomponent

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func Test(t *testing.T) {
	unittest.InitGinkgo(t, "Stable component Controller")
}

var _ = Describe("Stable component Controller", func() {
	Describe("", func() {
		It("should remove component from queue if desired version equal stable version ", func() {

			g := NewWithT(GinkgoT())

			c := controller{}

			redisName := "redis"
			wordpressName := "wordpress"
			mariadbName := "mariadb"

			queueRedis := s2hv1.Queue{
				Spec: s2hv1.QueueSpec{Name: redisName,
					Components: s2hv1.QueueComponents{{Name: redisName, Repository: redisName, Version: "1.0.1"}},
				},
			}
			queueMariadb := s2hv1.Queue{
				Spec: s2hv1.QueueSpec{Name: mariadbName,
					Components: s2hv1.QueueComponents{{Name: mariadbName, Repository: mariadbName, Version: "2.0.1"}},
				},
			}
			queueBundle := s2hv1.Queue{
				Spec: s2hv1.QueueSpec{Name: "db", Bundle: "group",
					Components: s2hv1.QueueComponents{
						{Name: mariadbName, Repository: mariadbName, Version: "2.0.1"},
						{Name: wordpressName, Repository: wordpressName, Version: "3.0.1"},
					},
				},
			}
			queueList := &s2hv1.QueueList{
				Items: []s2hv1.Queue{
					queueRedis, queueBundle,
				},
			}

			stableComponentRedis := &s2hv1.StableComponent{
				ObjectMeta: metav1.ObjectMeta{
					Name: redisName,
				},
				Spec: s2hv1.StableComponentSpec{
					Name:       redisName,
					Repository: redisName,
					Version:    "1.0.0",
				},
			}

			stableComponentWordpress := &s2hv1.StableComponent{
				ObjectMeta: metav1.ObjectMeta{
					Name: wordpressName,
				},
				Spec: s2hv1.StableComponentSpec{
					Name:       wordpressName,
					Repository: wordpressName,
					Version:    "2.0.0",
				},
			}

			desiredComponentRedis := &s2hv1.DesiredComponent{
				ObjectMeta: metav1.ObjectMeta{
					Name: redisName,
				},
				Spec: s2hv1.DesiredComponentSpec{
					Name:       redisName,
					Repository: redisName,
					Version:    "1.0.0",
				},
			}

			desiredComponentWordpress := &s2hv1.DesiredComponent{
				ObjectMeta: metav1.ObjectMeta{
					Name: wordpressName,
				},
				Spec: s2hv1.DesiredComponentSpec{
					Name:       wordpressName,
					Repository: wordpressName,
					Version:    "2.0.0",
				},
			}

			removeQueue, updateQueue := c.removeSameVersionQueue(queueList, stableComponentRedis, desiredComponentRedis)
			g.Expect(updateQueue).To(Equal(s2hv1.Queue{}))
			g.Expect(removeQueue).NotTo(Equal(s2hv1.Queue{}))
			g.Expect(removeQueue).To(Equal(queueRedis))

			removeQueue, updateQueue = c.removeSameVersionQueue(queueList, stableComponentWordpress, desiredComponentWordpress)
			g.Expect(updateQueue).NotTo(Equal(s2hv1.Queue{}))
			g.Expect(removeQueue).To(Equal(s2hv1.Queue{}))
			g.Expect(updateQueue.Spec.Components).To(Equal(queueMariadb.Spec.Components))

		})
	})
})
