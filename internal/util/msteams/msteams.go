package msteams

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
)

var logger = s2hlog.S2HLog.WithName("MS-Teams-util")

const requestTimeout = 5 * time.Second

const (
	tokenAPI       = "%s/%s/oauth2/v2.0/token"          // base login url, tenantID
	postMessageAPI = "%s/teams/%s/channels/%s/messages" // base graph url, groupID, channelID
	profileAPI     = "%s/me"                            // base graph url
	channelInfoAPI = "%s/teams/%s/channels/%s"          // base graph url, groupID, channelID
	joinedTeamsAPI = "%s/users/%s/joinedTeams"          // base graph url, userID
	channelsAPI    = "%s/teams/%s/channels"             // base graph url, groupID
)

// MSTeams is the interface of Microsoft Teams using Microsoft Graph api
type MSTeams interface {
	// GetAccessToken returns an access token on behalf of a user
	GetAccessToken() (string, error)

	//PostMessage posts message to the given Microsoft Teams group and channel
	PostMessage(groupID, channelID, message, accessToken string, opts ...PostMsgOption) error

	// GetGroupID returns group id from group name or id
	GetGroupID(groupNameOrID, accessToken string) (string, error)

	// GetGroupID returns channel id from channel name or id
	GetChannelID(groupID, channelNameOrID, accessToken string) (string, error)
}

var _ MSTeams = &Client{}

// Client manages client side of Microsoft Graph api
type Client struct {
	tenantID     string
	clientID     string
	clientSecret string
	username     string
	password     string

	contentType MessageContentType

	baseLoginURL string
	baseGraphURL string
}

// NewOption allows specifying various configuration
type NewOption func(*Client)

// WithBaseURL specifies base api url to override when creating Microsoft Teams Client
func WithBaseURL(baseLoginURL, baseGraphURL string) NewOption {
	return func(r *Client) {
		r.baseLoginURL = baseLoginURL
		r.baseGraphURL = baseGraphURL
	}
}

// NewClient creates a new client of MSTeams
func NewClient(tenantID, clientID, clientSecret, username, password string, opts ...NewOption) *Client {
	client := &Client{
		tenantID:     tenantID,
		clientID:     clientID,
		clientSecret: clientSecret,
		username:     username,
		password:     password,
		contentType:  PlainText,
		baseLoginURL: "https://login.microsoftonline.com",
		baseGraphURL: "https://graph.microsoft.com/beta",
	}

	// apply the new options
	for _, opt := range opts {
		opt(client)
	}

	return client
}

// GetAccessToken returns an access token on behalf of a user
func (c *Client) GetAccessToken() (string, error) {
	logger.Debug("getting Microsoft Teams access token")

	timeout := 5 * time.Second
	tokenAPI := fmt.Sprintf(tokenAPI, c.baseLoginURL, c.tenantID)

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

		_, res, err := postRequest(tokenAPI, []byte(reqBody.Encode()), opts...)
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

type messageReq struct {
	Body messageBody `json:"body"`
}

type messageBody struct {
	ContentType string `json:"contentType,omitempty"`
	Content     string `json:"content"`
}

// PostMsgOption allows specifying various configuration
type PostMsgOption func(*Client)

// MessageContentType defines a message content type
type MessageContentType string

const (
	PlainText MessageContentType = "text"
	HTML      MessageContentType = "html"
)

func WithContentType(contentType MessageContentType) PostMsgOption {
	return func(c *Client) {
		c.contentType = contentType
	}
}

// PostMessage implements the Microsoft Teams PostMessage function
func (c *Client) PostMessage(groupID, channelID, message, accessToken string, opts ...PostMsgOption) error {
	logger.Debug("Posting message", "groupID", groupID, "channelID", channelID)

	// apply the new options
	for _, opt := range opts {
		opt(c)
	}

	timeout := 10 * time.Second
	postMessageAPI := fmt.Sprintf(postMessageAPI, c.baseGraphURL, groupID, channelID)

	resCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()
	go func() {
		for {
			if accessToken == "" {
				var err error
				accessToken, err = c.GetAccessToken()
				if err != nil {
					errCh <- err
					return
				}
			}

			opts := []http.Option{
				http.WithTimeout(timeout),
				http.WithContext(ctx),
				http.WithHeader("Authorization", accessToken),
			}

			reqJSON := messageReq{
				Body: messageBody{
					ContentType: string(c.contentType),
					Content:     message,
				},
			}

			reqBody, err := json.Marshal(reqJSON)
			if err != nil {
				logger.Error(err, "cannot marshal request data", "data", reqBody)
				errCh <- err
				return
			}

			respCode, res, err := postRequest(postMessageAPI, reqBody, opts...)
			if err != nil {
				// reset access token if it's expired
				if respCode == 401 {
					accessToken = ""
					continue
				}

				errCh <- err
				return
			}

			resCh <- res
			return
		}
	}()

	select {
	case <-ctx.Done():
		logger.Error(s2herrors.ErrRequestTimeout,
			fmt.Sprintf("posting message to groupID: %s, channelID: %s took longer than %v",
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

// GetGroupID implements the Microsoft Teams GetGroupID function
func (c *Client) GetGroupID(groupNameOrID, accessToken string) (string, error) {
	if !validateGroupID(groupNameOrID) {
		return c.getMatchedGroupID(groupNameOrID, accessToken)
	}

	return groupNameOrID, nil
}

// GetChannelID implements the Microsoft Teams GetChannelID function
func (c *Client) GetChannelID(groupID, channelNameOrID, accessToken string) (string, error) {
	if err := c.getChannelInfo(groupID, channelNameOrID, accessToken); err != nil {
		return c.getMatchedChannelID(groupID, channelNameOrID, accessToken)
	}

	return channelNameOrID, nil
}

// getMatchedGroupID returns group id of the given group name
func (c *Client) getMatchedGroupID(groupName, accessToken string) (string, error) {
	timeout := 30 * time.Second

	userID, err := c.getMyUserID(accessToken)
	if err != nil {
		logger.Error(err, "cannot get user id of ms teams application")
		return "", err
	}

	groupIDCh := make(chan string, 1)
	errCh := make(chan error, 1)
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()

	go func() {
		nextLink := ""

		for {
			if accessToken == "" {
				var err error
				accessToken, err = c.GetAccessToken()
				if err != nil {
					errCh <- err
					return
				}
			}

			getGroupsAPI := nextLink
			if nextLink == "" {
				getGroupsAPI = fmt.Sprintf(joinedTeamsAPI, c.baseGraphURL, userID)
			}

			opts := []http.Option{
				http.WithTimeout(requestTimeout),
				http.WithContext(ctx),
				http.WithHeader("Authorization", accessToken),
			}

			respCode, res, err := getRequest(getGroupsAPI, opts...)
			if err != nil {
				// reset access token if it's expired
				if respCode == 401 {
					accessToken = ""
					continue
				}

				errCh <- err
				return
			}

			var respJSON struct {
				NextLink string `json:"@odata.nextLink,omitempty"`
				Values   []struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
				} `json:"value"`
			}
			if err := json.Unmarshal(res, &respJSON); err != nil {
				errCh <- err
				return
			}

			for _, group := range respJSON.Values {
				if strings.TrimSpace(groupName) == strings.TrimSpace(group.DisplayName) {
					groupIDCh <- group.ID
					return
				}
			}

			nextLink = respJSON.NextLink
			if nextLink == "" {
				errCh <- fmt.Errorf("group %s not found", groupName)
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		logger.Error(s2herrors.ErrRequestTimeout,
			fmt.Sprintf("getting joined team lists took longer than %v", requestTimeout))
		return "", s2herrors.ErrRequestTimeout
	case err := <-errCh:
		logger.Error(err, "cannot get joined team lists")
		return "", err
	case groupID := <-groupIDCh:
		return groupID, nil
	}
}

// getMatchedChannelID returns channel id of the given channel name
func (c *Client) getMatchedChannelID(groupID, channelName, accessToken string) (string, error) {
	timeout := 30 * time.Second

	channelIDCh := make(chan string, 1)
	errCh := make(chan error, 1)
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()

	go func() {
		nextLink := ""

		for {
			if accessToken == "" {
				var err error
				accessToken, err = c.GetAccessToken()
				if err != nil {
					errCh <- err
					return
				}
			}

			getChannelsAPI := nextLink
			if nextLink == "" {
				getChannelsAPI = fmt.Sprintf(channelsAPI, c.baseGraphURL, groupID)
			}

			opts := []http.Option{
				http.WithTimeout(requestTimeout),
				http.WithContext(ctx),
				http.WithHeader("Authorization", accessToken),
			}

			respCode, res, err := getRequest(getChannelsAPI, opts...)
			if err != nil {
				// reset access token if it's expired
				if respCode == 401 {
					accessToken = ""
					continue
				}

				errCh <- err
				return
			}

			var respJSON struct {
				NextLink string `json:"@odata.nextLink,omitempty"`
				Values   []struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
				} `json:"value"`
			}
			if err := json.Unmarshal(res, &respJSON); err != nil {
				errCh <- err
				return
			}

			for _, channel := range respJSON.Values {
				if strings.TrimSpace(channelName) == strings.TrimSpace(channel.DisplayName) {
					channelIDCh <- channel.ID
					return
				}
			}

			nextLink = respJSON.NextLink
			if nextLink == "" {
				errCh <- fmt.Errorf("channel %s not found", channelName)
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		logger.Error(s2herrors.ErrRequestTimeout,
			fmt.Sprintf("getting channels of groupID %s took longer than %v", groupID, requestTimeout))
		return "", s2herrors.ErrRequestTimeout
	case err := <-errCh:
		logger.Error(err, "cannot get channels")
		return "", err
	case channelID := <-channelIDCh:
		return channelID, nil
	}
}

func (c *Client) getMyUserID(accessToken string) (string, error) {
	logger.Debug("getting service account ID of MS Teams app")

	resCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	ctx, cancelFunc := context.WithTimeout(context.Background(), requestTimeout)
	defer cancelFunc()

	go func() {
		for {
			opts := []http.Option{
				http.WithTimeout(requestTimeout),
				http.WithContext(ctx),
				http.WithHeader("Authorization", accessToken),
			}

			profileAPI := fmt.Sprintf(profileAPI, c.baseGraphURL)
			respCode, res, err := getRequest(profileAPI, opts...)
			if err != nil {
				// reset access token if it's expired
				if respCode == 401 {
					accessToken = ""
					continue
				}

				errCh <- err
				return
			}

			resCh <- res
		}
	}()

	select {
	case <-ctx.Done():
		logger.Error(s2herrors.ErrRequestTimeout,
			fmt.Sprintf("getting user profile took longer than %v", requestTimeout))
		return "", s2herrors.ErrRequestTimeout
	case err := <-errCh:
		logger.Error(err, "cannot get user profile")
		return "", err
	case res := <-resCh:
		var respJSON struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(res, &respJSON); err != nil {
			logger.Error(err, "cannot unmarshal user profile json response")
			return "", err
		}

		return respJSON.ID, nil
	}
}

func (c *Client) getChannelInfo(groupID, channelNameOrID, accessToken string) error {
	channelInfoAPI := fmt.Sprintf(channelInfoAPI, c.baseGraphURL, groupID, channelNameOrID)

	resCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	ctx, cancelFunc := context.WithTimeout(context.Background(), requestTimeout)
	defer cancelFunc()

	go func() {
		for {
			if accessToken == "" {
				var err error
				accessToken, err = c.GetAccessToken()
				if err != nil {
					errCh <- err
					return
				}
			}

			opts := []http.Option{
				http.WithTimeout(requestTimeout),
				http.WithContext(ctx),
				http.WithHeader("Authorization", accessToken),
			}

			respCode, res, err := getRequest(channelInfoAPI, opts...)
			if err != nil {
				// reset access token if it's expired
				if respCode == 401 {
					accessToken = ""
					continue
				}

				errCh <- err
				return
			}

			resCh <- res
		}

	}()

	select {
	case <-ctx.Done():
		logger.Error(s2herrors.ErrRequestTimeout,
			fmt.Sprintf("getting channel info of groupID: %s, channel: %s took longer than %v",
				groupID, channelNameOrID, requestTimeout))
		return s2herrors.ErrRequestTimeout
	case err := <-errCh:
		return err
	case <-resCh:
		return nil
	}
}

func getRequest(reqURL string, opts ...http.Option) (int, []byte, error) {
	respCode, res, err := http.Get(reqURL, opts...)
	if err != nil {
		return respCode, []byte{}, err
	}

	return respCode, res, nil
}

func postRequest(reqURL string, body []byte, opts ...http.Option) (int, []byte, error) {
	respCode, res, err := http.Post(reqURL, body, opts...)
	if err != nil {
		return respCode, []byte{}, err
	}

	return respCode, res, nil
}

func validateGroupID(groupID string) bool {
	_, err := uuid.Parse(groupID)
	return err == nil
}
