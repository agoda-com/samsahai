package plugin

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestPluginplugin(t *testing.T) {
	unittest.InitGinkgo(t, "Plugin")
}

var _ = Describe("Plugin", func() {
	g := NewWithT(GinkgoT())

	pluginName := "example"

	It("should successfully load and verify plugin", func() {
		plugin, err := New("./example-shell.sh")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(plugin).NotTo(BeNil())

		name := plugin.GetName()
		g.Expect(name).To(Equal(pluginName))
	})

	Specify("Non-existing plugin path", func() {
		plugin, err := NewWithTimeout("./not-existing.sh", 1*time.Second, 1*time.Second, 1*time.Second)
		g.Expect(err).To(HaveOccurred())
		g.Expect(plugin).To(BeNil())
	})

	Describe("get-version", func() {
		It("should correctly get version", func() {
			plugin, err := New("./example-shell.sh")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(plugin).NotTo(BeNil())

			version, err := plugin.GetVersion("repo", "example", "")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(version).To(Equal("0.3.0"))

			version, err = plugin.GetVersion("repo", "example", "0\\.2\\..*")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(version).To(Equal("0.2.0"))

			version, err = plugin.GetVersion("repo", "example", "0\\.1\\..*")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(version).To(Equal("0.1.1"))

			err = plugin.EnsureVersion("repo", "example", "0\\.1\\.1")
			g.Expect(err).NotTo(HaveOccurred())
		})

		Specify("Timeout component", func() {
			plugin, err := NewWithTimeout("./example-shell.sh", 1*time.Second, 1*time.Second, 1*time.Second)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(plugin).NotTo(BeNil())

			_, err = plugin.GetVersion("repo", "timeout", ".*")
			g.Expect(err).To(HaveOccurred())
			g.Expect(err).To(Equal(errors.ErrRequestTimeout))

			_, err = plugin.GetVersion("repo", "fast-timeout", ".*")
			g.Expect(err).To(HaveOccurred())
			g.Expect(err).To(Equal(errors.ErrRequestTimeout))
		})

		Specify("Non-existing component", func() {
			plugin, err := NewWithTimeout("./example-shell.sh", 1*time.Second, 1*time.Second, 1*time.Second)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(plugin).NotTo(BeNil())

			_, err = plugin.GetVersion("repo", "not-found", "0\\.1\\..*")
			g.Expect(err).To(HaveOccurred())
			g.Expect(err).To(Equal(errors.ErrNoDesiredComponentVersion))

		})
	})

	Describe("get-component", func() {
		It("should successfully get component name", func() {
			plugin, err := New("./example-shell.sh")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(plugin).NotTo(BeNil())

			name := plugin.GetComponentName("test")
			Expect(name).To(Equal("test"))

			name = plugin.GetComponentName("Kubernetes")
			Expect(name).To(Equal("k8s"))
		})

		Specify("Timeout component", func() {
			plugin, err := NewWithTimeout("./example-shell.sh", 1*time.Second, 1*time.Second, 1*time.Second)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(plugin).NotTo(BeNil())

			name := plugin.GetComponentName("timeout")
			g.Expect(name).To(Equal("timeout"))
		})
	})
})
