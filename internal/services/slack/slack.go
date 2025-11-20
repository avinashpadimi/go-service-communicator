package slack

import (
	"fmt"
	"log"
	"strconv"
	"time"

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

// AuthTest calls the auth.test API method to get information about the bot.
func (c *Client) AuthTest() (*slack.AuthTestResponse, error) {
	log.Println("Calling Slack API: auth.test")
	return c.api.AuthTest()
}

// SendMessage sends a message to a Slack channel.
func (c *Client) SendMessage(channel, message string) error {
	log.Printf("Calling Slack API: chat.postMessage to channel %s", channel)
	_, _, err := c.api.PostMessage(channel, slack.MsgOptionText(message, false))
	return err
}

// GetConversationHistory fetches the conversation history from a channel.
func (c *Client) GetConversationHistory(channelID string, start, end time.Time) ([]string, error) {
	log.Printf("Calling Slack API: conversations.history for channel %s", channelID)
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Oldest:    strconv.FormatInt(start.Unix(), 10),
		Latest:    strconv.FormatInt(end.Unix(), 10),
	}

	history, err := c.api.GetConversationHistory(params)
	if err != nil {
		return nil, err
	}

	var messages []string
	for _, msg := range history.Messages {
		messages = append(messages, msg.Text)
	}

	return messages, nil
}

// GetPublicChannels fetches a list of all channels the bot is a member of, using cursor pagination.
func (c *Client) GetPublicChannels() ([]string, error) {
	log.Println("Calling Slack API: users.conversations with pagination")
	var allChannelIDs []string
	cursor := ""

	for {
		params := &slack.GetConversationsForUserParameters{
			ExcludeArchived: true,
			Types:           []string{"public_channel", "private_channel", "mpim", "im"}, // Fetching all types of conversations
			Cursor:          cursor,
			Limit:           100, // Fetch up to 100 channels per page
		}

		channels, nextCursor, err := c.api.GetConversationsForUser(params)
		if err != nil {
			return nil, fmt.Errorf("failed to get user conversations: %w", err)
		}

		for _, channel := range channels {
			// With users.conversations, all returned channels are ones the bot is a member of.
			// We can still log the name for clarity.
			log.Printf("Bot is a member of channel: %s (%s)", channel.Name, channel.ID)
			allChannelIDs = append(allChannelIDs, channel.ID)

			if len(allChannelIDs) >= 30 { // Limit to 30 channels
				break
			}
		}

		if len(allChannelIDs) >= 30 || nextCursor == "" { // Stop if 30 channels reached or no more pages
			break
		}
		cursor = nextCursor
	}

	return allChannelIDs, nil
}

// SearchMessages searches for messages matching a query.
func (c *Client) SearchMessages(query string) (*slack.SearchMessages, error) {
	log.Printf("Calling Slack API: search.messages with query '%s'", query)
	// Note: The empty string for sorting and the default pagination parameters are used.
	// For a more advanced implementation, these could be configurable.
	return c.api.SearchMessages(query, slack.SearchParameters{})
}

