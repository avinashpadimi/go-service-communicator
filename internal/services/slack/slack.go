package slack

import (
	"fmt"
	"time"
	"strconv"

	"github.com/slack-go/slack"
)

// Client is a Slack client that uses the slack-go library.
type Client struct {
	api *slack.Client
}

// New creates a new Slack client.
func New(token string) *Client {
	api := slack.New(token)
	return &Client{
		api: api,
	}
}

// SendMessage sends a message to a Slack channel.
func (c *Client) SendMessage(channel, message string) error {
	_, _, err := c.api.PostMessage(channel, slack.MsgOptionText(message, false))
	return err
}

// GetConversationHistory fetches the conversation history from a channel.
func (c *Client) GetConversationHistory(channelID string, start, end time.Time) ([]string, error) {
	fmt.Println("start and end time:", start.Unix(), end.Unix())
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Oldest:    strconv.FormatInt(start.Unix(), 10),
		Latest:    strconv.FormatInt(end.Unix(), 10),
	}

	history, err := c.api.GetConversationHistory(params)
	if err != nil {
		fmt.Println("error is :", err.Error())
		return nil, err
	}

	var messages []string
	for _, msg := range history.Messages {
		messages = append(messages, msg.Text)
	}

	fmt.Println("all messges is :", messages)
	return messages, nil
}

// GetPublicChannels fetches a list of all public channel IDs.
func (c *Client) GetPublicChannels() ([]string, error) {
	params := &slack.GetConversationsParameters{
		ExcludeArchived: true,
		Types:           []string{"public_channel", "private_channel", "mpim", "im"},
	}

	channels, _, err := c.api.GetConversations(params)
	for _, c := range channels {
		fmt.Println("channels info: ", c.Name)
	}
	if err != nil {
		return nil, err
	}

	var channelIDs []string
	for _, channel := range channels {
		channelIDs = append(channelIDs, channel.ID)
	}

	return channelIDs, nil
}

