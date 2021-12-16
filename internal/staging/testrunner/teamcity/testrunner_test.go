package teamcity_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agoda-com/samsahai/internal/staging/testrunner/teamcity"
	"github.com/agoda-com/samsahai/internal/util/unittest"
	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

func TestTeamcity(t *testing.T) {
	unittest.InitGinkgo(t, "Teamcity")
}

var _ = Describe("Teamcity Test Runner", func() {
	g := NewWithT(GinkgoT())

	var server *httptest.Server

	mockQueue := s2hv1.Queue{
		Spec: s2hv1.QueueSpec{
			Name: "test",
		},
		Status: s2hv1.QueueStatus{
			TestRunner: s2hv1.TestRunner{
				Teamcity: s2hv1.Teamcity{
					BuildID: "1234",
				},
			},
		},
	}

	mockTestConfig := s2hv1.ConfigTestRunner{
		Teamcity: &s2hv1.ConfigTeamcity{
			Branch: "<default>",
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
<?xml version="1.0" encoding="UTF-8" standalone="yes"?><build id="1234567890"></build>
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			testConfig := mockTestConfig
			currentQueue := mockQueue

			tcRunner := teamcity.New(nil, server.URL, "", "")
			err := tcRunner.Trigger(&testConfig, &currentQueue)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(currentQueue.Status.TestRunner.Teamcity.Branch).To(Equal(testConfig.Teamcity.Branch))
			g.Expect(currentQueue.Status.TestRunner.Teamcity.BuildID).To(Equal("1234567890"))
		})

		It("should successfully trigger test with PR data rendering", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`
<?xml version="1.0" encoding="UTF-8" standalone="yes"?><build id="1234567890"></build>
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			prNumber := "1234"
			testConfig := mockTestConfig
			testConfig.Teamcity.Branch = "pull/{{ .PRNumber }}"

			currentQueue := mockQueue
			currentQueue.Spec.PRNumber = prNumber

			tcRunner := teamcity.New(nil, server.URL, "", "")
			err := tcRunner.Trigger(&testConfig, &currentQueue)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(currentQueue.Status.TestRunner.Teamcity.Branch).To(Equal("pull/1234"))
			g.Expect(currentQueue.Status.TestRunner.Teamcity.BuildID).To(Equal("1234567890"))
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

			tcRunner := teamcity.New(nil, server.URL, "", "")
			err := tcRunner.Trigger(&testConfig, &currentQueue)
			g.Expect(err).NotTo(BeNil())
		})
	})

	Describe("Get Result", func() {
		It("should successfully get test result with running state", func(done Done) {
			defer close(done)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				_, err := ioutil.ReadAll(r.Body)
				g.Expect(err).NotTo(HaveOccurred())

				_, err = w.Write([]byte(`
<?xml version="1.0" encoding="UTF-8" standalone="yes"?><build state="running"></build>
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			testConfig := mockTestConfig
			currentQueue := mockQueue

			tcRunner := teamcity.New(nil, server.URL, "", "")
			isSuccess, isFinished, err := tcRunner.GetResult(&testConfig, &currentQueue)
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
<?xml version="1.0" encoding="UTF-8" standalone="yes"?><build state="finished" status="success"></build>
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			testConfig := mockTestConfig
			currentQueue := mockQueue

			tcRunner := teamcity.New(nil, server.URL, "", "")
			isSuccess, isFinished, err := tcRunner.GetResult(&testConfig, &currentQueue)
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
<?xml version="1.0" encoding="UTF-8" standalone="yes"?><build state="finished" status="failure"></build>
`))
				g.Expect(err).NotTo(HaveOccurred())
			}))
			defer server.Close()

			testConfig := mockTestConfig
			currentQueue := mockQueue

			tcRunner := teamcity.New(nil, server.URL, "", "")
			isSuccess, isFinished, err := tcRunner.GetResult(&testConfig, &currentQueue)
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

			tcRunner := teamcity.New(nil, server.URL, "", "")
			isSuccess, isFinished, err := tcRunner.GetResult(&testConfig, &currentQueue)
			g.Expect(err).NotTo(BeNil())
			g.Expect(isFinished).To(BeFalse())
			g.Expect(isSuccess).To(BeFalse())
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
			currentQueue.Status.TestRunner.Teamcity.BuildID = ""

			tcRunner := teamcity.New(nil, server.URL, "", "")
			isSuccess, isFinished, err := tcRunner.GetResult(&testConfig, &currentQueue)
			g.Expect(err.Error()).To(BeEquivalentTo(errors.Wrapf(s2herrors.ErrTestPipelineIDNotFound,
				"cannot get test result. buildId: '%s'. queue: %s",
				currentQueue.Status.TestRunner.Teamcity.BuildID,
				currentQueue.Name).Error(),
			))
			g.Expect(isFinished).To(BeTrue())
			g.Expect(isSuccess).To(BeFalse())
		})
	})
})
