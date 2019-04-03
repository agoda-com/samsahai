package rest_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
	"github.com/tidwall/gjson"

	"github.com/agoda-com/samsahai/internal/samsahai/component"

	"github.com/agoda-com/samsahai/internal/samsahai/reporter/rest"
	. "github.com/onsi/gomega"
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

func TestRest_SendMessage(t *testing.T) {
	g := NewGomegaWithT(t)
	msg := "hello-world"
	_type := "message"
	server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
		g.Expect(json.Valid(body)).To(BeTrue(), "request body should be json string")
		var data interface{}
		err := json.Unmarshal(body, &data)
		g.Expect(err).To(BeNil(), "should successfully unmarshalling byte to interface{}")
		m := data.(map[string]interface{})
		g.Expect(m["event_type"]).To(Equal(_type), "event_type should be 'message'")
		g.Expect(m["message"]).To(Equal(msg), "message should matched")
	})
	defer server.Close()
	client := rest.NewRest(server.URL)
	err := client.SendMessage(msg)
	g.Expect(err).To(BeNil(), "request should not thrown any error")
}

func TestRest_SendMessageFailed(t *testing.T) {
	g := NewGomegaWithT(t)
	server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
		g.Expect(json.Valid(body)).To(BeTrue(), "request body should be json string")
		var data interface{}
		err := json.Unmarshal(body, &data)
		g.Expect(err).To(BeNil(), "should successfully unmarshalling byte to interface{}")
		res.WriteHeader(400)
	})
	defer server.Close()
	client := rest.NewRest(server.URL)
	err := client.SendMessage("hello-world")
	g.Expect(err).NotTo(BeNil(), "request should thrown an error")
}

func TestRest_SendImageMissingList(t *testing.T) {
	g := NewGomegaWithT(t)
	eventType := "missing_image"
	images := []component.Image{
		{Repository: "docker.io/hello-world", Tag: "v1.0.0"},
		{Repository: "docker.io/hello-a", Tag: "2018.01.01"},
	}
	server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
		g.Expect(json.Valid(body)).To(BeTrue(), "request body should be json string")
		var data interface{}
		err := json.Unmarshal(body, &data)
		g.Expect(err).To(BeNil(), "should successfully unmarshalling byte to interface{}")
		m := data.(map[string]interface{})
		g.Expect(m["event_type"]).To(Equal(eventType), "event_type should be 'message'")
		g.Expect(m["images"]).To(BeAssignableToTypeOf([]interface{}{}))
		images := m["images"].([]interface{})
		g.Expect(reflect.DeepEqual(images[0], map[string]interface{}{"repository": "docker.io/hello-world", "tag": "v1.0.0"})).To(BeTrue(),
			fmt.Sprintf("mismatch json: %v", images[0]))
		g.Expect(reflect.DeepEqual(images[1], map[string]interface{}{"repository": "docker.io/hello-a", "tag": "2018.01.01"})).To(BeTrue(),
			fmt.Sprintf("mismatch json: %v", images[1]))
	})
	defer server.Close()
	client := rest.NewRest(server.URL)
	err := client.SendImageMissingList(images)
	g.Expect(err).To(BeNil(), "request should not thrown any error")
}

func TestRest_SendOutdatedComponents(t *testing.T) {
	g := NewGomegaWithT(t)
	event := "outdated_components"
	outdatedComs := []component.OutdatedComponent{
		{
			CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
			NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
			OutdatedDays:     1,
		},
		{
			CurrentComponent: &component.Component{Name: "comp2", Version: "2018.1.1"},
			NewComponent:     &component.Component{Name: "comp2", Version: "2018.1.2"},
			OutdatedDays:     2,
		},
	}

	server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
		g.Expect(json.Valid(body)).To(BeTrue(), "request body should be json string")
		var data interface{}
		err := json.Unmarshal(body, &data)
		g.Expect(err).To(BeNil(), "should successfully unmarshalling byte to interface{}")

		_event := gjson.GetBytes(body, "event_type")
		g.Expect(_event.String()).To(Equal(event), "event_type should be match")
		_outdated := gjson.GetBytes(body, "outdated_components")
		g.Expect(_outdated.Exists()).To(BeTrue(), "outdated_components keys should exist")
		g.Expect(_outdated.IsArray()).To(BeTrue(), "outdated_components should be an array")
		g.Expect(_outdated.Array()[0].Raw).To(Equal(
			`{"current_component":{"name":"comp1","version":"1.1.0"},"new_component":{"name":"comp1","version":"1.1.2"},"outdated_days":1}`),
			"json should match")
		g.Expect(_outdated.Array()[1].Raw).To(Equal(
			`{"current_component":{"name":"comp2","version":"2018.1.1"},"new_component":{"name":"comp2","version":"2018.1.2"},"outdated_days":2}`),
			"json should match")
	})
	defer server.Close()
	client := rest.NewRest(server.URL)
	err := client.SendOutdatedComponents(outdatedComs)
	g.Expect(err).To(BeNil(), "request should not thrown any error")
}

func TestRest_SendActivePromotionStatus(t *testing.T) {
	g := NewGomegaWithT(t)

	event := "active_promotion"
	status := "success"
	namespace := "ns-1"
	serviceOwner := "abcd"
	outdatedComs := []component.OutdatedComponent{
		{
			CurrentComponent: &component.Component{Name: "comp1", Version: "1.1.0"},
			NewComponent:     &component.Component{Name: "comp1", Version: "1.1.2"},
			OutdatedDays:     1,
		},
		{
			CurrentComponent: &component.Component{Name: "comp2", Version: "2018.1.1"},
			NewComponent:     &component.Component{Name: "comp2", Version: "2018.1.1"},
			OutdatedDays:     0,
		},
	}

	server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
		g.Expect(gjson.ValidBytes(body)).To(BeTrue(), "request body should be json")
		g.Expect(gjson.GetBytes(body, "event_type").String()).To(Equal(event),
			"event_type should be match")
		g.Expect(gjson.GetBytes(body, "service_owner").String()).To(Equal(serviceOwner),
			"service_owner should be match")
		g.Expect(gjson.GetBytes(body, "status").String()).To(Equal(status),
			"status should be match")
		g.Expect(gjson.GetBytes(body, "active_namespace").String()).To(Equal(namespace),
			"active_namespace should be match")

		_outdated := gjson.GetBytes(body, "outdated_components")
		g.Expect(_outdated.Exists()).To(BeTrue(), "outdated_components keys should exist")
		g.Expect(_outdated.IsArray()).To(BeTrue(), "outdated_components should be an array")
		g.Expect(_outdated.Array()[0].Raw).To(Equal(
			`{"current_component":{"name":"comp1","version":"1.1.0"},"new_component":{"name":"comp1","version":"1.1.2"},"outdated_days":1}`),
			"json should match")
		g.Expect(_outdated.Array()[1].Raw).To(Equal(
			`{"current_component":{"name":"comp2","version":"2018.1.1"},"new_component":{"name":"comp2","version":"2018.1.1"},"outdated_days":0}`),
			"json should match")
	})
	defer server.Close()

	client := rest.NewRest(server.URL)
	err := client.SendActivePromotionStatus(status, namespace, serviceOwner, outdatedComs)
	g.Expect(err).To(BeNil(), "request should not thrown any error")
}

func TestRest_SendComponentUpgradeFail(t *testing.T) {
	g := NewGomegaWithT(t)

	event := "component_upgrade_failed"
	serviceOwner := "abcd"
	opts := []reporter.Option{
		reporter.NewOptionValuesFileURL("url-to-values-file"),
		reporter.NewOptionIssueType("verify failed"),
		reporter.NewOptionCIURL("url-to-ci"),
		reporter.NewOptionErrorURL("url-to-error"),
		reporter.NewOptionLogsURL("url-to-logs"),
	}

	com := &component.Component{
		Name:    "comp1",
		Version: "1.1.0",
		Image:   &component.Image{Repository: "image-1", Tag: "1.1.0"},
	}

	server := newServer(g, func(res http.ResponseWriter, req *http.Request, body []byte) {
		g.Expect(gjson.ValidBytes(body)).To(BeTrue(), "request body should be json")
		g.Expect(gjson.GetBytes(body, "event_type").String()).To(Equal(event),
			"event_type should be match")
		g.Expect(gjson.GetBytes(body, "service_owner").String()).To(Equal(serviceOwner),
			"service_owner should be match")
		g.Expect(gjson.GetBytes(body, "issue_type").String()).To(Equal("verify failed"),
			"issue_type should be match")

		c := gjson.GetBytes(body, "component")
		g.Expect(c.Exists()).To(BeTrue(), "component keys should exist")
		g.Expect(c.Raw).To(Equal(
			`{"name":"comp1","version":"1.1.0","image":{"repository":"image-1","tag":"1.1.0"}}`),
			"component json should match")
		urls := gjson.GetBytes(body, "urls")
		g.Expect(urls.Exists()).To(BeTrue(), "urls keys should exist")
		g.Expect(urls.Get("values_file").String()).To(Equal(`url-to-values-file`), "values_file should match")
		g.Expect(urls.Get("logs").String()).To(Equal(`url-to-logs`), "logs should match")
		g.Expect(urls.Get("error").String()).To(Equal(`url-to-error`), "error should match")
		g.Expect(urls.Get("ci").String()).To(Equal(`url-to-ci`), "ci should match")
	})
	defer server.Close()

	client := rest.NewRest(server.URL)
	err := client.SendComponentUpgradeFail(com, serviceOwner, opts...)
	g.Expect(err).To(BeNil(), "request should not thrown any error")
}
