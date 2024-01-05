package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"
)

// Client manages client side of http request
type Client struct {
	BaseURL  *url.URL
	client   *http.Client
	req      *http.Request
	username string
	password string
}

type Option func(client *Client)

func WithSkipTLSVerify() Option {
	return func(c *Client) {
		c.client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.client.Timeout = timeout
	}
}

func WithContext(ctx context.Context) Option {
	return func(c *Client) {
		if c.req == nil {
			return
		}
		c.req = c.req.WithContext(ctx)
	}
}

func WithHeader(key, value string) Option {
	return func(c *Client) {
		if c.req == nil {
			return
		}
		c.req.Header.Set(key, value)
	}
}

func WithBasicAuth(username, password string) Option {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

// NewClient creates http client
func NewClient(baseURL string, opts ...Option) *Client {
	var err error
	var client = Client{
		client: &http.Client{},
	}

	client.BaseURL, err = url.Parse(baseURL)
	if err != nil {
		log.Fatalln("invalid url")
	}

	for _, opt := range opts {
		opt(&client)
	}

	return &client
}

// Delete sends http delete
func (c *Client) Delete(reqURI string, opts ...Option) (int, []byte, error) {
	baseURL, err := url.Parse(c.BaseURL.String())
	if err != nil {
		return 0, nil, err
	}
	baseURL.Path = path.Join(baseURL.Path, reqURI)
	req, err := http.NewRequest("DELETE", baseURL.String(), nil)
	if err != nil {
		return 0, nil, err
	}
	for _, opt := range opts {
		opt(c)
	}
	return c.request(req)
}

// Post sends http post
func (c *Client) Post(reqURI string, data []byte, opts ...Option) (int, []byte, error) {
	baseURL, err := url.Parse(c.BaseURL.String())
	if err != nil {
		return 0, nil, err
	}

	baseURL.Path = path.Join(baseURL.Path, reqURI)

	req, err := http.NewRequest("POST", baseURL.String(), bytes.NewBuffer(data))

	if err != nil {
		return 0, nil, err
	}

	for _, opt := range opts {
		opt(c)
	}

	return c.request(req)
}

// Get sends http get
func (c *Client) Get(reqURI string, opts ...Option) (int, []byte, error) {
	baseURL, err := url.Parse(c.BaseURL.String())
	if err != nil {
		return 0, nil, err
	}

	baseURL.Path = path.Join(baseURL.Path, reqURI)

	req, err := http.NewRequest("GET", baseURL.String(), nil)
	if err != nil {
		return 0, nil, err
	}

	for _, opt := range opts {
		opt(c)
	}

	return c.request(req)
}

func Get(reqURI string, opts ...Option) (int, []byte, error) {
	var err error
	var client = Client{
		client: &http.Client{},
	}

	reqURL, err := url.Parse(reqURI)
	if err != nil {
		return 0, nil, err
	}

	client.req, err = http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return 0, nil, err
	}

	for _, opt := range opts {
		opt(&client)
	}

	return client.request(client.req)
}

// Post sends http post
func Post(reqURI string, data []byte, opts ...Option) (int, []byte, error) {
	var err error
	var client = Client{
		client: &http.Client{},
	}

	reqURL, err := url.Parse(reqURI)
	if err != nil {
		return 0, nil, err
	}

	client.req, err = http.NewRequest("POST", reqURL.String(), bytes.NewBuffer(data))
	if err != nil {
		return 0, nil, err
	}

	for _, opt := range opts {
		opt(&client)
	}

	return client.request(client.req)
}

// Delete sends http delete
func Delete(reqURI string, opts ...Option) (int, []byte, error) {
	var err error
	var client = Client{
		client: &http.Client{},
	}
	reqURL, err := url.Parse(reqURI)
	if err != nil {
		return 0, nil, err
	}
	client.req, err = http.NewRequest("DELETE", reqURL.String(), nil)
	if err != nil {
		return 0, nil, err
	}
	for _, opt := range opts {
		opt(&client)
	}
	return client.request(client.req)
}

func (c *Client) request(req *http.Request) (int, []byte, error) {
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, body, nil
	}

	return resp.StatusCode, nil, fmt.Errorf("%d - %s", resp.StatusCode, body)
}
