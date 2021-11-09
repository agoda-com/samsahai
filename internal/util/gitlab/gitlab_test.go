package gitlab_test

import (
	"github.com/agoda-com/samsahai/internal/util/gitlab"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestGitlab(t *testing.T) {
	unittest.InitGinkgo(t, "Gitlab Util")
}

var _ = Describe("Gitlab REST API", func() {
	g := NewWithT(GinkgoT())

	var gitlabClient *gitlab.Client

	var server *httptest.Server

	var (
		token      = "sometoken"
		repository = "samsahai/samsahai"
	)

	Describe("PublishCommitStatus", func() {
		var (
			commitSHA   = "3bb4cd1d909cdfa804de5bde2defa144b066d36c"
			labelName   = "test"
			targetURL   = "url"
			description = "description"
			status      = gitlab.CommitStatusSuccess
		)

		It("should successfully publish commit status for a given SHA", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`
{
    "id": 888523,
    "sha": "3bb4cd1d909cdfa804de5bde2defa144b066d36c",
    "ref": "refs/merge-requests/356/head",
    "status": "success",
    "name": "default",
    "target_url": null,
    "description": "description",
    "created_at": "2021-11-08T06:27:57.587Z",
    "started_at": null,
    "finished_at": "2021-11-08T06:27:57.585Z",
    "allow_failure": false,
    "coverage": null,
    "author": {
        "id": 1,
        "name": "octocat",
        "username": "octocat",
        "state": "active",
        "avatar_url": "https://gitlab.com/images/error/octocat_happy.gif",
        "web_url": "https://gitlab.com/octocat"
    }
}
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			gitlabClient = gitlab.NewClient(server.URL, token)
			err := gitlabClient.PublishCommitStatus(repository, commitSHA, labelName, targetURL, description, status)
			g.Expect(err).NotTo(HaveOccurred())
		})

		Specify("Invalid json response", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				w.WriteHeader(400)
				_, err = w.Write([]byte(``))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			gitlabClient = gitlab.NewClient(server.URL, token)
			err := gitlabClient.PublishCommitStatus("", "", "", "",
				"", "")
			g.Expect(err).NotTo(BeNil())
		})
	})
})
