package gitlab_test

import (
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/gitlab"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestGitlab(t *testing.T) {
	unittest.InitGinkgo(t, "GitLab")
}

var _ = Describe("GitLab", func() {
	g := NewWithT(GinkgoT())

	var server *httptest.Server

	mockQueue := s2hv1.Queue{
		Spec: s2hv1.QueueSpec{
			Name: "test",
		},
		Status: s2hv1.QueueStatus{
			TestRunner: s2hv1.TestRunner{
				Gitlab: s2hv1.Gitlab{
					Branch:         "main",
					PipelineID:     "1111",
					PipelineURL:    "",
					PipelineNumber: "",
				},
			},
		},
	}

	mockTestConfig := s2hv1.ConfigTestRunner{
		Gitlab: &s2hv1.ConfigGitlab{
			ProjectID:            "1234",
			Branch:               "main",
			PipelineTriggerToken: "testpipelineTriggertoken",
		},
	}

	Describe("Trigger", func() {
		It("should successfully trigger test", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`
{"id": 4321,"web_url": "https://gitlab.com/test/-/pipelines/4321"}
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			testConfig := mockTestConfig
			currentQueue := mockQueue

			glRunner := gitlab.New(nil, server.URL)
			err := glRunner.Trigger(&testConfig, &currentQueue)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(currentQueue.Status.TestRunner.Gitlab.PipelineID).To(Equal("4321"))
			g.Expect(currentQueue.Status.TestRunner.Gitlab.PipelineURL).To(Equal("https://gitlab.com/test/-/pipelines/4321"))
		})

		It("should successfully trigger test with PR data rendering", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`
{"id": 4321,"web_url": "https://gitlab.com/test/-/pipelines/4321"}
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			prNumber := "1234"
			testConfig := mockTestConfig

			currentQueue := mockQueue
			currentQueue.Spec.PRNumber = prNumber

			glRunner := gitlab.New(nil, server.URL)
			err := glRunner.Trigger(&testConfig, &currentQueue)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(currentQueue.Status.TestRunner.Gitlab.PipelineID).To(Equal("4321"))
			g.Expect(currentQueue.Status.TestRunner.Gitlab.PipelineURL).To(Equal("https://gitlab.com/test/-/pipelines/4321"))
		})

		Specify("Invalid json response", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(``))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			testConfig := mockTestConfig
			currentQueue := mockQueue

			glRunner := gitlab.New(nil, server.URL)
			err := glRunner.Trigger(&testConfig, &currentQueue)
			g.Expect(err).NotTo(BeNil())
		})

		Describe("Get Result", func() {
			It("should successfully get test result with running state", func(done Done) {
				defer close(done)
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()
					_, err := ioutil.ReadAll(r.Body)
					g.Expect(err).NotTo(HaveOccurred())

					_, err = w.Write([]byte(`
{"status": "running","started_at": "2021-04-09T20:36:27.710Z"}
`))
					g.Expect(err).NotTo(HaveOccurred())
				}))
				defer server.Close()

				testConfig := mockTestConfig
				currentQueue := mockQueue

				glRunner := gitlab.New(nil, server.URL)
				isSuccess, isFinished, err := glRunner.GetResult(&testConfig, &currentQueue)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(isFinished).To(BeFalse())
				g.Expect(isSuccess).To(BeFalse())
			})

			It("should successfully get test result with success status", func(done Done) {
				defer close(done)
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()
					_, err := ioutil.ReadAll(r.Body)
					g.Expect(err).NotTo(HaveOccurred())

					_, err = w.Write([]byte(`
{"status": "success","started_at": "2021-04-09T20:36:27.710Z","finished_at": "2021-04-09T20:36:51.368Z"}
`))
					g.Expect(err).NotTo(HaveOccurred())
				}))
				defer server.Close()

				testConfig := mockTestConfig
				currentQueue := mockQueue

				glRunner := gitlab.New(nil, server.URL)
				isSuccess, isFinished, err := glRunner.GetResult(&testConfig, &currentQueue)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(isFinished).To(BeTrue())
				g.Expect(isSuccess).To(BeTrue())
			})

			It("should successfully get test result with failure status", func(done Done) {
				defer close(done)
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()
					_, err := ioutil.ReadAll(r.Body)
					g.Expect(err).NotTo(HaveOccurred())

					_, err = w.Write([]byte(`
{"status": "failed","started_at": "2021-04-09T20:36:27.710Z","finished_at": "2021-04-09T20:36:51.368Z"}
`))
					g.Expect(err).NotTo(HaveOccurred())
				}))
				defer server.Close()

				testConfig := mockTestConfig
				currentQueue := mockQueue

				glRunner := gitlab.New(nil, server.URL)
				isSuccess, isFinished, err := glRunner.GetResult(&testConfig, &currentQueue)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(isFinished).To(BeTrue())
				g.Expect(isSuccess).To(BeFalse())
			})

			Specify("Invalid json response", func(done Done) {
				defer close(done)
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()
					_, err := ioutil.ReadAll(r.Body)
					g.Expect(err).NotTo(HaveOccurred())

					_, err = w.Write([]byte(``))
					g.Expect(err).NotTo(HaveOccurred())
				}))
				defer server.Close()

				testConfig := mockTestConfig
				currentQueue := mockQueue

				glRunner := gitlab.New(nil, server.URL)
				_, _, err := glRunner.GetResult(&testConfig, &currentQueue)
				g.Expect(err).NotTo(BeNil())
			})

			Specify("Trigger test fail, then pipelineID not found", func(done Done) {
				defer close(done)
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()
					_, err := ioutil.ReadAll(r.Body)
					g.Expect(err).NotTo(HaveOccurred())

					_, err = w.Write([]byte(``))
					g.Expect(err).NotTo(HaveOccurred())
				}))
				defer server.Close()

				testConfig := mockTestConfig
				currentQueue := mockQueue
				currentQueue.Status.TestRunner.Gitlab.PipelineID = ""

				glRunner := gitlab.New(nil, server.URL)
				isSuccess, isFinished, err := glRunner.GetResult(&testConfig, &currentQueue)
				g.Expect(err).NotTo(BeNil())
				g.Expect(err.Error()).To(BeEquivalentTo(errors.Wrapf(s2herrors.ErrTestPipelineIDNotFound,
					"cannot get test result. pipelineID: '%s'. queue: %s",
					currentQueue.Status.TestRunner.Gitlab.PipelineID,
					currentQueue.Name).Error(),
				))
				g.Expect(isFinished).To(BeTrue())
				g.Expect(isSuccess).To(BeFalse())
			})

		})
	})
})
