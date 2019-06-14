package agodacspider_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/desiredcomponent/checker/agodacspider"
)

func TestChecker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "agoda cspider checker")
}

var _ = Describe("checker", func() {
	if os.Getenv("DEBUG") != "" {
		log.SetLogger(log.ZapLogger(true))
	}

	var checker internal.DesiredComponentChecker
	//var err error
	var accessToken = "123456"

	It("should returns non-empty name", func() {
		viper.Set(agodacspider.EnvURL, "http://127.0.0.1")
		viper.Set(agodacspider.EnvAccessToken, accessToken)
		checker = agodacspider.New()
		Expect(checker).NotTo(BeNil())
		Expect(checker.GetName()).NotTo(BeEmpty())
	})

	Describe("validation", func() {
		It("should error if no url provided", func() {
			viper.Set(agodacspider.EnvURL, "")
			viper.Set(agodacspider.EnvAccessToken, accessToken)
			checker = agodacspider.New()
			Expect(checker).To(BeNil())
		})
		It("should error if no access token provided", func() {
			viper.Set(agodacspider.EnvURL, "localhost")
			viper.Set(agodacspider.EnvAccessToken, "")
			checker = agodacspider.New()
			Expect(checker).To(BeNil())
		})
	})

	Describe("bad path", func() {
		BeforeEach(func() {
			viper.Set(agodacspider.EnvAccessToken, accessToken)
		})

		Specify("no response", func() {
			server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				GinkgoRecover()
				_, err := ioutil.ReadAll(req.Body)
				Expect(err).To(BeNil())
				defer req.Body.Close()

				_, _ = res.Write([]byte(``))
			}))
			defer server.Close()

			viper.Set(agodacspider.EnvURL, server.URL)
			checker = agodacspider.New()
			Expect(checker).NotTo(BeNil())

			version, err := checker.GetVersion("", "maxwell", "")
			Expect(err).NotTo(BeNil())
			Expect(version).To(BeEmpty())
		})

		Specify("invalid json response", func() {
			server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				GinkgoRecover()
				_, err := ioutil.ReadAll(req.Body)
				Expect(err).To(BeNil())
				defer req.Body.Close()

				_, _ = res.Write([]byte(`{
    "status": true,
    "message": "Success",
    "stackTrace": null,
    "data": "",
    "error": 0
}`))
			}))
			defer server.Close()

			viper.Set(agodacspider.EnvURL, server.URL)
			checker = agodacspider.New()
			Expect(checker).NotTo(BeNil())

			version, err := checker.GetVersion("", "maxwell", "")
			Expect(err).NotTo(BeNil())
			Expect(version).To(BeEmpty())
		})

		Specify("no version", func() {
			server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				GinkgoRecover()
				_, err := ioutil.ReadAll(req.Body)
				Expect(err).To(BeNil())
				defer req.Body.Close()

				_, _ = res.Write([]byte(`{
    "status": true,
    "message": "Success",
    "stackTrace": null,
    "data": "[{\"app\":\"Maxwell App\",\"server\":\"hk-hsn-2001\",\"dc\":\"HKG\",\"version\":\"maxwell.zip\"}]",
    "error": 0
}`))
			}))
			defer server.Close()

			viper.Set(agodacspider.EnvURL, server.URL)
			checker = agodacspider.New()
			Expect(checker).NotTo(BeNil())

			version, err := checker.GetVersion("", "maxwell", "")
			Expect(err.Error()).To(Equal(internal.ErrNoDesiredComponentVersion.Error()))
			Expect(version).To(BeEmpty())
		})
	})

	Describe("good path", func() {
		BeforeEach(func() {
			viper.Set(agodacspider.EnvAccessToken, accessToken)
		})

		It("should correctly extract version", func() {
			server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				GinkgoRecover()
				_, err := ioutil.ReadAll(req.Body)
				Expect(err).To(BeNil())
				defer req.Body.Close()

				_, _ = res.Write([]byte(`{
    "status": true,
    "message": "Success",
    "stackTrace": null,
    "data": "[{\"app\":\"Maxwell App\",\"server\":\"hk-hsn-2001\",\"dc\":\"HKG\",\"version\":\"maxwell_1.0.270.zip\"}]",
    "error": 0
}`))
			}))
			defer server.Close()

			viper.Set(agodacspider.EnvURL, server.URL)
			checker = agodacspider.New()
			Expect(checker).NotTo(BeNil())

			version, err := checker.GetVersion("", "maxwell", "")
			Expect(err).To(BeNil())
			Expect(version).To(Equal("1.0.270"))
		})
	})

	It("should correctly return majority version", func() {
		server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			GinkgoRecover()
			_, err := ioutil.ReadAll(req.Body)
			Expect(err).To(BeNil())
			defer req.Body.Close()
			versions := []string{
				"maxwell_1.0.274.zip",
				"maxwell_1.0.270.zip",
				"maxwell_1.0.273.zip",
				"maxwell_1.0.271.zip",
				"maxwell_1.0.272.zip",
				"maxwell_1.0.273.zip",
				"N/A",
				"N/A",
				"N/A",
			}
			for i, v := range versions {
				versions[i] = `{\"app\":\"Maxwell App\",\"server\":\"hk-hsn-2001\",\"dc\":\"HKG\",\"version\":\"` + v + `\"}`
			}
			data := strings.Join(versions, ",")

			_, _ = res.Write([]byte(`{
    "status": true,
    "message": "Success",
    "stackTrace": null,
    "data": "[` + data + `]",
    "error": 0
}`))
		}))
		defer server.Close()

		viper.Set(agodacspider.EnvURL, server.URL)
		checker = agodacspider.New()
		Expect(checker).NotTo(BeNil())

		version, err := checker.GetVersion("", "maxwell", "")
		Expect(err).To(BeNil())
		Expect(version).To(Equal("1.0.273"))
	})

	It("should correctly return latest version", func() {
		server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			GinkgoRecover()
			_, err := ioutil.ReadAll(req.Body)
			Expect(err).To(BeNil())
			defer req.Body.Close()
			versions := []string{
				"maxwell_1.0.271.zip",
				"maxwell_1.0.272.zip",
				"maxwell_1.0.273.zip",
				"maxwell_1.0.270.zip",
				"N/A",
				"N/A",
				"N/A",
			}
			for i, v := range versions {
				versions[i] = `{\"app\":\"Maxwell App\",\"server\":\"hk-hsn-2001\",\"dc\":\"HKG\",\"version\":\"` + v + `\"}`
			}
			data := strings.Join(versions, ",")

			_, _ = res.Write([]byte(`{
    "status": true,
    "message": "Success",
    "stackTrace": null,
    "data": "[` + data + `]",
    "error": 0
}`))
		}))
		defer server.Close()

		viper.Set(agodacspider.EnvURL, server.URL)
		checker = agodacspider.New()
		Expect(checker).NotTo(BeNil())

		version, err := checker.GetVersion("", "maxwell", "")
		Expect(err).To(BeNil())
		Expect(version).To(Equal("1.0.273"))
	})
})
