package msteams

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
)

var logger = s2hlog.S2HLog.WithName("MS-Teams-util")

const requestTimeout = 5 * time.Second

const (
	tokenAPI       = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
	postMessageAPI = "https://graph.microsoft.com/beta/teams/%s/channels/%s/messages"
)

// MSTeams is the interface of Microsoft Teams using Microsoft Graph api
type MSTeams interface {
	//PostMessage posts message to the given Microsoft Teams group and channel
	PostMessage(groupID, channelID, message string) error
}

var _ MSTeams = &Client{}

// Client manages client side of Microsoft Graph api
type Client struct {
	tenantID     string
	clientID     string
	clientSecret string
	username     string
	password     string
}

// NewClient creates a new client of MSTeams
func NewClient(tenantID, clientID, clientSecret, username, password string) *Client {
	client := Client{
		tenantID:     tenantID,
		clientID:     clientID,
		clientSecret: clientSecret,
		username:     username,
		password:     password,
	}

	return &client
}

type messageReq struct {
	Body messageBody `json:"body"`
}

type messageBody struct {
	Content string `json:"content"`
}

// PostMessage implements the Microsoft Teams PostMessage function
func (c *Client) PostMessage(groupID, channelID, message string) error {
	logger.Debug("Posting message", "groupID", groupID, "channelID", channelID)

	timeout := 10 * time.Second
	postMessageAPI := fmt.Sprintf(postMessageAPI, groupID, channelID)

	resCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()

	go func() {
		accessToken, err := c.getAccessToken()
		if err != nil {
			errCh <- err
			return
		}

		opts := []http.Option{
			http.WithTimeout(timeout),
			http.WithContext(ctx),
			http.WithHeader("Authorization", accessToken),
		}

		reqJson := messageReq{
			Body: messageBody{Content: message},
		}

		reqBody, err := json.Marshal(reqJson)
		if err != nil {
			logger.Error(err, "cannot marshal request data", "data", reqBody)
			errCh <- err
			return
		}

		res, err := post(postMessageAPI, reqBody, opts...)
		if err != nil {
			errCh <- err
			return
		}

		resCh <- res
	}()

	select {
	case <-ctx.Done():
		logger.Error(s2herrors.ErrRequestTimeout,
			fmt.Sprintf("posting message to group: %s, channel: %s took longer than %v",
				groupID, channelID, requestTimeout))
		return s2herrors.ErrRequestTimeout
	case err := <-errCh:
		logger.Error(err, "cannot post message", "groupID", groupID, "channelID", channelID)
		return err
	case <-resCh:
		logger.Info("message successfully sent to channel",
			"groupID", groupID, "channelID", channelID)
		return nil
	}
}

// getAccessToken returns an access token on behalf of a user
func (c *Client) getAccessToken() (string, error) {
	logger.Debug("getting MS Teams access token")

	timeout := 5 * time.Second
	tokenAPI := fmt.Sprintf(tokenAPI, c.tenantID)

	resCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()

	go func() {
		opts := []http.Option{
			http.WithTimeout(timeout),
			http.WithContext(ctx),
			http.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		}

		reqBody := url.Values{}
		reqBody.Set("grant_type", "password")
		reqBody.Set("client_id", c.clientID)
		reqBody.Set("client_secret", c.clientSecret)
		reqBody.Set("scope", "https://graph.microsoft.com/.default")
		reqBody.Set("userName", c.username)
		reqBody.Set("password", c.password)

		res, err := post(tokenAPI, []byte(reqBody.Encode()), opts...)
		if err != nil {
			errCh <- err
			return
		}

		resCh <- res
	}()

	select {
	case <-ctx.Done():
		logger.Error(s2herrors.ErrRequestTimeout,
			fmt.Sprintf("getting access token took longer than %v", requestTimeout))
		return "", s2herrors.ErrRequestTimeout
	case err := <-errCh:
		logger.Error(err, "cannot get access token")
		return "", err
	case res := <-resCh:
		var respJSON struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(res, &respJSON); err != nil {
			logger.Error(err, "cannot unmarshal access token json response")
			return "", err
		}

		return respJSON.AccessToken, nil
	}
}

func post(reqURL string, body []byte, opts ...http.Option) ([]byte, error) {
	res, err := http.Post(reqURL, body, opts...)
	if err != nil {
		logger.Error(err, "POST request failed", "url", reqURL, "body", string(body))
		return []byte{}, err
	}

	return res, nil
}
