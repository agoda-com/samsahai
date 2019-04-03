package http

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"
)

// Client manages client side of http request
type Client struct {
	BaseURL *url.URL
}

// NewClient creates http client
func NewClient(baseURL string) *Client {
	var (
		client Client
		err    error
	)
	client.BaseURL, err = url.Parse(baseURL)
	if err != nil {
		log.Fatalln("invalid url")
	}

	return &client
}

// Post sends http post
func (c *Client) Post(reqURI string, data []byte) ([]byte, error) {
	baseURL, err := url.Parse(c.BaseURL.String())
	if err != nil {
		return nil, err
	}

	baseURL.Path = path.Join(baseURL.Path, reqURI)

	req, err := http.NewRequest("POST", baseURL.String(), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	log.Printf("Data successfully sent to %s\n", baseURL.String())
	return c.request(req)
}

// Get sends http get
func (c *Client) Get(reqURI string) ([]byte, error) {
	baseURL, err := url.Parse(c.BaseURL.String())
	if err != nil {
		return nil, err
	}

	baseURL.Path = path.Join(baseURL.Path, reqURI)

	req, err := http.NewRequest("GET", baseURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return c.request(req)
}

func (c *Client) request(req *http.Request) ([]byte, error) {
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 25 * time.Minute,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 200 {
		return body, nil
	}

	return nil, Error(resp.StatusCode)
}
