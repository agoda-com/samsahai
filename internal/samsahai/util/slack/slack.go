package slack

import (
	"log"

	"github.com/nlopes/slack"
)

// Slack is the interface of slack
type Slack interface {
	// PostMessage posts message to slack channel
	PostMessage(channelNameOrID, message, username string) (channelID, timestamp string, err error)
}

var _ Slack = &Client{}

// Client manages client side of slack api
type Client struct {
	api *slack.Client
}

// NewClient creates a new client
func NewClient(token string) *Client {
	api := slack.New(token)
	client := Client{
		api: api,
	}

	return &client
}

// PostMessage implements the slack PostMessage function
func (c *Client) PostMessage(channelNameOrID, message, username string) (channelID, timestamp string, err error) {
	channelID, timestamp, err = c.api.PostMessage(channelNameOrID, slack.MsgOptionText(message, false), slack.MsgOptionUsername(username))
	if err != nil {
		return "", "", err
	}

	log.Printf("Message successfully sent to channel %s at %s\n", channelID, timestamp)
	return
}
