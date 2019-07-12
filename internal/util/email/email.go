package email

import (
	"log"

	"gopkg.in/gomail.v2"
)

// Email is the interface of email
type Email interface {
	// SendMessage sends message via email
	SendMessage(from string, to []string, subject, body string) error
}

var _ Email = &Client{}

// Client manages client side of mail server
type Client struct {
	dial    *gomail.Dialer
	message *gomail.Message
}

// NewClient creates a new client
func NewClient(server string, port int) *Client {
	client := Client{
		dial:    &gomail.Dialer{Host: server, Port: port},
		message: gomail.NewMessage(),
	}

	return &client
}

// SendMessage implements the email SendMessage function
func (c *Client) SendMessage(from string, to []string, subject, body string) error {
	c.message.SetHeader("From", from)
	c.message.SetHeader("To", to...)
	c.message.SetHeader("Subject", subject)
	c.message.SetBody("text/html", body)

	if err := c.dial.DialAndSend(c.message); err != nil {
		return err
	}

	log.Printf("Message successfully sent to %v\n", to)
	return nil
}
