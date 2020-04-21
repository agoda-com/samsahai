package slack

import (
	"github.com/nlopes/slack"

	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.S2HLog.WithName("Slack-util")

// Slack is the interface of slack
type Slack interface {
	// PostMessage posts message to slack channel
	PostMessage(channelNameOrID, message string, opts ...slack.MsgOption) error
}

var _ Slack = &Client{}

// Client manages client side of slack api
type Client struct {
	api *slack.Client
}

// NewClient creates a new client
func NewClient(token string) *Client {
	client := Client{
		api: slack.New(token),
	}

	return &client
}

// PostMessage implements the slack PostMessage function
func (c *Client) PostMessage(channelNameOrID, message string, opts ...slack.MsgOption) error {
	opts = append(opts, slack.MsgOptionText(message, false))

	_, _, err := c.api.PostMessage(
		channelNameOrID,
		opts...,
	)
	if err != nil {
		return err
	}

	logger.Info("message successfully sent to channel", "channel", channelNameOrID)
	return nil
}
