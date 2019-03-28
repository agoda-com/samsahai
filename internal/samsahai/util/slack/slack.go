package slack

import (
	"log"

	"github.com/nlopes/slack"
)

type Client struct {
	api *slack.Client
}

func NewClient(token string) *Client {
	api := slack.New(token)
	client := Client{
		api: api,
	}

	return &client
}

func (c *Client) PostMessage(channelNameOrID, message, username string) (channelID, timestamp string, err error) {
	channelID, timestamp, err = c.api.PostMessage(channelNameOrID, slack.MsgOptionText(message, false), slack.MsgOptionUsername(username))
	if err != nil {
		return "", "", err
	}

	log.Printf("Message successfully sent to channel %s at %s\n", channelID, timestamp)
	return
}
