package harbor

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hhttp "github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestHarborChecker(t *testing.T) {
	unittest.InitGinkgo(t, "Harbor Checker")
}

var _ = Describe("Harbor Checker", func() {
	g := NewWithT(GinkgoT())

	var checker internal.DesiredComponentChecker
	//var err error
	//var accessToken = "123456"
	var server *httptest.Server

	BeforeEach(func() {
		checker = New(s2hhttp.WithSkipTLSVerify())
	})

	AfterEach(func() {

	})

	It("should returns 'harbor' as name", func() {
		Expect(checker.GetName()).To(Equal("harbor"))
	})

	It("should successfully get new version from harbor", func(done Done) {
		defer close(done)
		server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_, err := ioutil.ReadAll(r.Body)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = w.Write([]byte(`
[
  {
    "digest": "sha256:84a2ffe970a4dba1786a5ca8adec9d7f70319f73ea13c0eef9fdf599f7fc1d10",
    "extra_attrs": {
      "architecture": "amd64",
      "author": null,
      "created": "2021-03-08T05:07:20.562882143Z",
      "os": "linux"
    },
    "icon": "sha256:0048162a053eef4d4ce3fe7518615bef084403614f8bca43b40ae2e762e11e06",
    "id": 11213,
    "labels": null,
    "manifest_media_type": "application/vnd.docker.distribution.manifest.v2+json",
    "media_type": "application/vnd.docker.container.image.v1+json",
    "project_id": 22,
    "pull_time": "2021-03-10T01:37:28.277Z",
    "push_time": "2021-03-09T03:18:09.374Z",
    "references": null,
    "repository_id": 1752,
    "size": 61224083,
    "tags": [
      {
        "artifact_id": 11213,
        "id": 10563,
        "immutable": false,
        "name": "1.13.5-3.0.0-beta.1",
        "pull_time": "0001-01-01T00:00:00.000Z",
        "push_time": "2021-03-09T03:18:09.429Z",
        "repository_id": 1752,
        "signed": false
      }
    ],
    "type": "IMAGE"
  },
  {
    "digest": "sha256:d4cc6c8a109215a954279d13c0d4b50f9ac714600d5897fa02310132c7c25f1d",
    "extra_attrs": {
      "architecture": "amd64",
      "author": null,
      "created": "2021-03-01T07:40:46.655920063Z",
      "os": "linux"
    },
    "icon": "sha256:0048162a053eef4d4ce3fe7518615bef084403614f8bca43b40ae2e762e11e06",
    "id": 7090,
    "labels": null,
    "manifest_media_type": "application/vnd.docker.distribution.manifest.v2+json",
    "media_type": "application/vnd.docker.container.image.v1+json",
    "project_id": 22,
    "pull_time": "2021-03-08T02:22:46.827Z",
    "push_time": "2021-03-01T15:29:27.792Z",
    "references": null,
    "repository_id": 1752,
    "size": 60768721,
    "tags": [
      {
        "artifact_id": 7090,
        "id": 7627,
        "immutable": false,
        "name": "1.13.5-3.0.0-alpha.1",
        "pull_time": "0001-01-01T00:00:00.000Z",
        "push_time": "2021-03-01T15:29:27.843Z",
        "repository_id": 1752,
        "signed": false
      }
    ],
    "type": "IMAGE"
  },
  {
    "digest": "sha256:ac83188223e3fc65fd04f2d2a1de04f9f4ea6e84cf36aa21a24fd9ff82eead28",
    "extra_attrs": {
      "architecture": "amd64",
      "author": null,
      "created": "2021-02-28T11:06:09.1846778Z",
      "os": "linux"
    },
    "icon": "sha256:0048162a053eef4d4ce3fe7518615bef084403614f8bca43b40ae2e762e11e06",
    "id": 6414,
    "labels": null,
    "manifest_media_type": "application/vnd.docker.distribution.manifest.v2+json",
    "media_type": "application/vnd.docker.container.image.v1+json",
    "project_id": 22,
    "pull_time": "2021-03-04T02:38:40.016Z",
    "push_time": "2021-02-28T17:56:55.699Z",
    "references": null,
    "repository_id": 1752,
    "size": 59566991,
    "tags": [
      {
        "artifact_id": 6414,
        "id": 7126,
        "immutable": false,
        "name": "v1.9.10",
        "pull_time": "0001-01-01T00:00:00.000Z",
        "push_time": "2021-02-28T17:56:55.749Z",
        "repository_id": 1752,
        "signed": false
      }
    ],
    "type": "IMAGE"
  },
  {
    "digest": "sha256:ff6aaa687b9be44e38475177bc601eec07c33c60828d382114e0fa596ff82271",
    "extra_attrs": {
      "architecture": "amd64",
      "author": null,
      "created": "2021-02-28T09:33:05.6696384Z",
      "os": "linux"
    },
    "icon": "sha256:0048162a053eef4d4ce3fe7518615bef084403614f8bca43b40ae2e762e11e06",
    "id": 6413,
    "labels": null,
    "manifest_media_type": "application/vnd.docker.distribution.manifest.v2+json",
    "media_type": "application/vnd.docker.container.image.v1+json",
    "project_id": 22,
    "pull_time": "2021-02-28T17:56:58.283Z",
    "push_time": "2021-02-28T17:56:55.045Z",
    "references": null,
    "repository_id": 1752,
    "size": 59518140,
    "tags": [
      {
        "artifact_id": 6413,
        "id": 7125,
        "immutable": false,
        "name": "1.13.5-3.0.0-beta.2",
        "pull_time": "0001-01-01T00:00:00.000Z",
        "push_time": "2021-02-28T17:56:55.110Z",
        "repository_id": 1752,
        "signed": false
      }
    ],
    "type": "IMAGE"
  }
]
`))
			g.Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		repo := strings.Replace(server.URL, "https://", "", 1) + "/aiab/kubectl"
		version, err := checker.GetVersion(repo, "kubectl", "1\\.13\\..+")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(version).To(Equal("1.13.5-3.0.0-beta.2"))
	})

	It("should correctly ensure version", func(done Done) {
		defer close(done)
		server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_, err := ioutil.ReadAll(r.Body)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = w.Write([]byte(`
[
  {
    "digest": "sha256:84a2ffe970a4dba1786a5ca8adec9d7f70319f73ea13c0eef9fdf599f7fc1d10",
    "extra_attrs": {
      "architecture": "amd64",
      "author": null,
      "created": "2021-03-08T05:07:20.562882143Z",
      "os": "linux"
    },
    "icon": "sha256:0048162a053eef4d4ce3fe7518615bef084403614f8bca43b40ae2e762e11e06",
    "id": 11213,
    "labels": null,
    "manifest_media_type": "application/vnd.docker.distribution.manifest.v2+json",
    "media_type": "application/vnd.docker.container.image.v1+json",
    "project_id": 22,
    "pull_time": "2021-03-10T01:37:28.277Z",
    "push_time": "2021-03-09T03:18:09.374Z",
    "references": null,
    "repository_id": 1752,
    "size": 61224083,
    "tags": [
      {
        "artifact_id": 11213,
        "id": 10563,
        "immutable": false,
        "name": "1.13.5-3.0.0-beta.1",
        "pull_time": "0001-01-01T00:00:00.000Z",
        "push_time": "2021-03-09T03:18:09.429Z",
        "repository_id": 1752,
        "signed": false
      }
    ],
    "type": "IMAGE"
  },
  {
    "digest": "sha256:d4cc6c8a109215a954279d13c0d4b50f9ac714600d5897fa02310132c7c25f1d",
    "extra_attrs": {
      "architecture": "amd64",
      "author": null,
      "created": "2021-03-01T07:40:46.655920063Z",
      "os": "linux"
    },
    "icon": "sha256:0048162a053eef4d4ce3fe7518615bef084403614f8bca43b40ae2e762e11e06",
    "id": 7090,
    "labels": null,
    "manifest_media_type": "application/vnd.docker.distribution.manifest.v2+json",
    "media_type": "application/vnd.docker.container.image.v1+json",
    "project_id": 22,
    "pull_time": "2021-03-08T02:22:46.827Z",
    "push_time": "2021-03-01T15:29:27.792Z",
    "references": null,
    "repository_id": 1752,
    "size": 60768721,
    "tags": [
      {
        "artifact_id": 7090,
        "id": 7627,
        "immutable": false,
        "name": "1.13.5-3.0.0-alpha.1",
        "pull_time": "0001-01-01T00:00:00.000Z",
        "push_time": "2021-03-01T15:29:27.843Z",
        "repository_id": 1752,
        "signed": false
      }
    ],
    "type": "IMAGE"
  }
]
`))
			g.Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		repo := strings.Replace(server.URL, "https://", "", 1) + "/aiab/kubectl"
		err := checker.EnsureVersion(repo, "kubectl", "1.13.5-3.0.0-beta.1")
		g.Expect(err).NotTo(HaveOccurred())

		err = checker.EnsureVersion(repo, "kubectl", "1.13.5-3.0.0-beta.1-missing")
		g.Expect(s2herrors.IsImageNotFound(err)).To(BeTrue())
	})

	Specify("Invalid json response", func(done Done) {
		defer close(done)
		server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_, err := ioutil.ReadAll(r.Body)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = w.Write([]byte(``))
			g.Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		repo := strings.Replace(server.URL, "https://", "", 1) + "/aiab/kubectl"
		_, err := checker.GetVersion(repo, "kubectl", "1\\.13\\..+")
		g.Expect(err).NotTo(BeNil())
	})
})
