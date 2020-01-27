package rest_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/reporter/rest"
	"github.com/agoda-com/samsahai/internal/util/unittest"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

type fn func(http.ResponseWriter, *http.Request, []byte)

func newServer(g *GomegaWithT, fnCheck fn) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		g.Expect(err).To(BeNil())
		defer req.Body.Close()
		g.Expect(req.Header.Get("Content-Type")).To(Equal("application/json"), "request should send application/json")
		fnCheck(resp, req, body)
	}))
	return server
}

func TestUnit(t *testing.T) {
	unittest.InitGinkgo(t, "Rest Reporter")
}

var _ = Describe("send rest message", func() {
	g := NewGomegaWithT(GinkgoT())

	Describe("success path", func() {
		It("should correctly send active promotion", func() {
			var comp1, repoComp1, comp2, repoComp2 = "comp1", "repo/comp1", "comp2", "repo/comp2"
			var v110, v112, v201811 = "1.1.0", "1.1.2", "2018.1.1"

			status := &s2hv1beta1.ActivePromotionStatus{
				Result:               s2hv1beta1.ActivePromotionSuccess,
				HasOutdatedComponent: true,
				OutdatedComponents: []*s2hv1beta1.OutdatedComponent{
					{
						Name:             comp1,
						CurrentImage:     &s2hv1beta1.Image{Repository: repoComp1, Tag: v110},
						LatestImage:      &s2hv1beta1.Image{Repository: repoComp1, Tag: v112},
						OutdatedDuration: time.Duration(86400000000000), // 1d0h0m
					},
					{
						Name:             comp2,
						CurrentImage:     &s2hv1beta1.Image{Repository: repoComp2, Tag: v201811},
						LatestImage:      &s2hv1beta1.Image{Repository: repoComp2, Tag: v201811},
						OutdatedDuration: time.Duration(0),
					},
				},
			}
			atpRpt := internal.NewActivePromotionReporter(status, internal.SamsahaiConfig{}, "owner", "owner-123456")

			server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
				g.Expect(gjson.ValidBytes(body)).To(BeTrue(), "request body should be json")
				g.Expect(gjson.GetBytes(body, "unixTimestamp").Exists()).To(BeTrue(),
					"unixTimestamp keys should exist")
				g.Expect(gjson.GetBytes(body, "uuid").Exists()).To(BeTrue(), "uuid keys should exist")
				g.Expect(gjson.GetBytes(body, "teamName").String()).To(Equal(atpRpt.TeamName),
					"teamName should be matched")
				g.Expect(gjson.GetBytes(body, "result").String()).To(Equal(string(atpRpt.Result)),
					"status should be matched")
				g.Expect(gjson.GetBytes(body, "currentActiveNamespace").String()).To(Equal(atpRpt.CurrentActiveNamespace),
					"active_namespace should be match")

				_outdated := gjson.GetBytes(body, "outdatedComponents")
				g.Expect(_outdated.Exists()).To(BeTrue(), "outdatedComponents keys should exist")
				g.Expect(_outdated.IsArray()).To(BeTrue(), "outdatedComponents should be an array")
				g.Expect(_outdated.Array()[0].Raw).To(Equal(
					`{"name":"comp1","currentImage":{"repository":"repo/comp1","tag":"1.1.0"},"latestImage":{"repository":"repo/comp1","tag":"1.1.2"},"outdatedDuration":86400000000000}`),
					"json should be matched")
				g.Expect(_outdated.Array()[1].Raw).To(Equal(
					`{"name":"comp2","currentImage":{"repository":"repo/comp2","tag":"2018.1.1"},"latestImage":{"repository":"repo/comp2","tag":"2018.1.1"},"outdatedDuration":0}`),
					"json should be matched")
			})
			defer server.Close()
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			client := rest.New(rest.WithRestClient(rest.NewRest(server.URL)))
			err := client.SendActivePromotionStatus(configMgr, atpRpt)
			g.Expect(err).To(BeNil(), "request should not thrown any error")
		})

		It("should correctly send component upgrade", func() {
			img := &rpc.Image{Repository: "image-1", Tag: "1.1.0"}
			rpcComp := &rpc.ComponentUpgrade{
				Name:       "comp1",
				Status:     rpc.ComponentUpgrade_UpgradeStatus_FAILURE,
				Image:      img,
				TeamName:   "owner",
				IssueType:  rpc.ComponentUpgrade_IssueType_IMAGE_MISSING,
				Namespace:  "owner-staging",
				IsReverify: true,
			}

			buildTypeID := "Teamcity_BuildTypeID"
			testRunner := s2hv1beta1.TestRunner{Teamcity: s2hv1beta1.Teamcity{BuildTypeID: buildTypeID}}
			comp := internal.NewComponentUpgradeReporter(
				rpcComp,
				internal.SamsahaiConfig{},
				internal.WithTestRunner(testRunner),
			)

			server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
				g.Expect(gjson.ValidBytes(body)).To(BeTrue(), "request body should be json")
				g.Expect(gjson.GetBytes(body, "unixTimestamp").Exists()).To(BeTrue(),
					"unixTimestamp keys should exist")
				g.Expect(gjson.GetBytes(body, "teamName").String()).To(Equal(rpcComp.TeamName),
					"teamName should be matched")
				g.Expect(gjson.GetBytes(body, "issueType").String()).To(Equal("Image missing"),
					"issueType should be matched")
				g.Expect(gjson.GetBytes(body, "component").String()).To(Equal(rpcComp.Name),
					"component should be matched")
				g.Expect(gjson.GetBytes(body, "testBuildTypeID").String()).To(Equal("teamcity_buildtypeid"),
					"testBuildTypeID should be matched")
				g.Expect(gjson.GetBytes(body, "imageRepository").String()).To(Equal(img.Repository),
					"imageRepository should be matched")
				g.Expect(gjson.GetBytes(body, "imageTag").String()).To(Equal(img.Tag),
					"imageTag should be matched")
				g.Expect(gjson.GetBytes(body, "namespace").String()).To(Equal(rpcComp.Namespace),
					"namespace should be matched")
				g.Expect(gjson.GetBytes(body, "teamcityURL").String()).To(BeEmpty(),
					"teamcityURL should be empty")
				g.Expect(gjson.GetBytes(body, "isReverify").String()).To(Equal("true"),
					"isReverify should be matched")
			})

			defer server.Close()
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			client := rest.New(rest.WithRestClient(rest.NewRest(server.URL)))
			err := client.SendComponentUpgrade(configMgr, comp)
			g.Expect(err).To(BeNil(), "request should not thrown any error")
		})

		It("should correctly send image missing", func() {
			img := &rpc.Image{Repository: "docker.io/hello-a", Tag: "2018.01.01"}
			server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
				g.Expect(gjson.ValidBytes(body)).To(BeTrue(), "request body should be json")
				g.Expect(gjson.GetBytes(body, "unixTimestamp").Exists()).To(BeTrue(),
					"unixTimestamp keys should exist")
				g.Expect(gjson.GetBytes(body, "uuid").Exists()).To(BeTrue(), "uuid keys should exist")
				g.Expect(gjson.GetBytes(body, "repository").String()).To(Equal("docker.io/hello-a"),
					"repository should be matched")
				g.Expect(gjson.GetBytes(body, "tag").String()).To(Equal("2018.01.01"),
					"tag should be matched")
			})

			defer server.Close()
			configMgr := newConfigMock()
			g.Expect(configMgr).ShouldNot(BeNil())

			client := rest.New(rest.WithRestClient(rest.NewRest(server.URL)))
			err := client.SendImageMissing(configMgr, img)
			g.Expect(err).To(BeNil(), "request should not thrown any error")
		})
	})

	Describe("failure path", func() {
		It("should fail to send message", func() {
			server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
				res.WriteHeader(400)
			})
			defer server.Close()

			client := rest.New(rest.WithRestClient(rest.NewRest(server.URL)))
			configMgr := newConfigMock()

			err := client.SendComponentUpgrade(configMgr, &internal.ComponentUpgradeReporter{})
			g.Expect(err).NotTo(BeNil(), "component upgrade request should thrown an error")

			err = client.SendActivePromotionStatus(configMgr, &internal.ActivePromotionReporter{})
			g.Expect(err).NotTo(BeNil(), "active promotion request should thrown an error")

			err = client.SendImageMissing(configMgr, &rpc.Image{})
			g.Expect(err).NotTo(BeNil(), "image missing request should thrown an error")
		})

		It("should not send message if not define rest reporter configuration", func() {
			calls := 0
			server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
				g.Expect(json.Valid(body)).To(BeTrue(), "request body should be json string")
				var data interface{}
				err := json.Unmarshal(body, &data)
				g.Expect(err).To(BeNil(), "should successfully unmarshalling byte to interface{}")

				calls++
				res.WriteHeader(200)
			})
			defer server.Close()

			client := rest.New(rest.WithRestClient(rest.NewRest(server.URL)))
			configMgr := newNoRestConfig()

			err := client.SendComponentUpgrade(configMgr, &internal.ComponentUpgradeReporter{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))

			err = client.SendActivePromotionStatus(configMgr, &internal.ActivePromotionReporter{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))

			err = client.SendImageMissing(configMgr, &rpc.Image{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(calls).To(Equal(0))
		})
	})

})

func newConfigMock() internal.ConfigManager {
	return config.NewWithBytes([]byte(`
report:
  rest:
    componentUpgrade:
      endpoints:
        - url: http://resturl
          template: ./testdata/component-upgrade-failure.tpl
    activePromotion:
      endpoints:
        - url: http://resturl
    imageMissing:
      endpoints:
        - url: http://resturl
`))
}

func newNoRestConfig() internal.ConfigManager {
	configMgr := config.NewWithBytes([]byte(`
report:
`))

	return configMgr
}
