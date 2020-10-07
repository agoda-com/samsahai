package teamcity_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal/staging/testrunner/teamcity"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestTeamcity(t *testing.T) {
	unittest.InitGinkgo(t, "Teamcity")
}

var _ = Describe("Teamcity Test Runner", func() {
	g := NewWithT(GinkgoT())

	var server *httptest.Server

	mockQueue := s2hv1beta1.Queue{
		Spec: s2hv1beta1.QueueSpec{
			Name: "test",
		},
	}

	mockTestConfig := s2hv1beta1.ConfigTestRunner{
		Teamcity: &s2hv1beta1.ConfigTeamcity{
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
			_, _, err := tcRunner.GetResult(&testConfig, &currentQueue)
			g.Expect(err).NotTo(BeNil())
		})
	})
})
