package github_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal/util/github"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestGithub(t *testing.T) {
	unittest.InitGinkgo(t, "Github Util")
}

var _ = Describe("Github REST API", func() {
	g := NewWithT(GinkgoT())

	var githubClient *github.Client

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
			status      = github.CommitStatusSuccess
		)

		It("should successfully publish commit status for a given SHA", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`
{
  "url": "https://api.github.com/repos/octocat/Hello-World/statuses/6dcb09b5b57875f334f61aebed695e2e4193db5e",
  "avatar_url": "https://github.com/images/error/hubot_happy.gif",
  "id": 1,
  "node_id": "MDY6U3RhdHVzMQ==",
  "state": "success",
  "description": "Build has completed successfully",
  "target_url": "https://ci.example.com/1000/output",
  "context": "continuous-integration/jenkins",
  "created_at": "2012-07-20T01:19:13Z",
  "updated_at": "2012-07-20T01:19:13Z",
  "creator": {
    "login": "octocat",
    "id": 1,
    "node_id": "MDQ6VXNlcjE=",
    "avatar_url": "https://github.com/images/error/octocat_happy.gif",
    "gravatar_id": "",
    "url": "https://api.github.com/users/octocat",
    "html_url": "https://github.com/octocat",
    "followers_url": "https://api.github.com/users/octocat/followers",
    "following_url": "https://api.github.com/users/octocat/following{/other_user}",
    "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
    "organizations_url": "https://api.github.com/users/octocat/orgs",
    "repos_url": "https://api.github.com/users/octocat/repos",
    "events_url": "https://api.github.com/users/octocat/events{/privacy}",
    "received_events_url": "https://api.github.com/users/octocat/received_events",
    "type": "User",
    "site_admin": false
  }
}
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			githubClient = github.NewClient(server.URL, token)
			err := githubClient.PublishCommitStatus(repository, commitSHA, labelName, targetURL, description, status)
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

			githubClient = github.NewClient(server.URL, token)
			err := githubClient.PublishCommitStatus("", "", "", "",
				"", "")
			g.Expect(err).NotTo(BeNil())
		})
	})
})
