package git

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestGitClient(t *testing.T) {
	unittest.InitGinkgo(t, "Git Client")
}

var _ = Describe("Git Client", func() {
	t := NewGomegaWithT(GinkgoT())

	It("Should return correct path", func() {
		git, err := NewClient("samsahai", "https://github.com/agoda-com/samsahai.git")
		t.Expect(err).NotTo(HaveOccurred())
		t.Expect(git.GetPath()).To(Equal("agoda-com/samsahai"))
		t.Expect(git.GetName()).To(Equal("samsahai"))

		git, err = NewClient("samsahai", "git@github.com:agoda-com/samsahai.git")
		t.Expect(err).NotTo(HaveOccurred())
		t.Expect(git.GetPath()).To(Equal("agoda-com/samsahai"))
		t.Expect(git.GetName()).To(Equal("samsahai"))

		git, err = NewClient("samsahai", "git@github.com:samsahai.git")
		t.Expect(err).NotTo(HaveOccurred())
		t.Expect(err).NotTo(HaveOccurred())
		t.Expect(git.GetPath()).To(Equal("samsahai"))
		t.Expect(git.GetName()).To(Equal("samsahai"))
	})

})
