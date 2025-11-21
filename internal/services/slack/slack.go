package slack

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"
)

// Client is a Slack client that uses the slack-go library.
type Client struct {
	api          *slack.Client
	userCache    map[string]string
	channelCache map[string]string
	cacheMutex   sync.Mutex
}

// New creates a new Slack client.
func New(token string) *Client {
	api := slack.New(token)
	return &Client{
		api:          api,
		userCache:    make(map[string]string),
		channelCache: make(map[string]string),
	}
}

// AuthTest calls the auth.test API method to get information about the bot.
func (c *Client) AuthTest() (*slack.AuthTestResponse, error) {
	log.Println("Calling Slack API: auth.test")
	return c.api.AuthTest()
}

// SendMessage sends a message to a Slack channel using blocks.
func (c *Client) SendMessage(channel, message string) error {
	log.Printf("Calling Slack API: chat.postMessage to channel %s", channel)

	// Try to unmarshal the message as Slack message blocks
	var blocks slack.Blocks
	err := json.Unmarshal([]byte(message), &blocks)
	if err == nil {
		// If unmarshalling succeeds, send the blocks.
		_, _, postErr := c.api.PostMessage(channel, slack.MsgOptionBlocks(blocks.BlockSet...))
		return postErr
	}

	// If unmarshalling fails, assume it's a plain text message and use formatText.
	log.Printf("Could not unmarshal message as JSON blocks, formatting as plain text: %v", err)
	formattedBlocks := c.formatText(message)
	_, _, postErr := c.api.PostMessage(
		channel,
		slack.MsgOptionBlocks(formattedBlocks...),
	)
	return postErr
}

func (c *Client) formatText(message string) []slack.Block {
	var blocks []slack.Block
	lines := strings.Split(message, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var textObj *slack.TextBlockObject

		// Detect headings (lines starting with "#")
		if strings.HasPrefix(line, "#") {
			heading := strings.TrimLeft(line, "# ")
			textObj = slack.NewTextBlockObject("mrkdwn", "*"+heading+"*", false, false)
		} else if strings.HasPrefix(line, "```") {
			// Code block lines (preserve exactly)
			textObj = slack.NewTextBlockObject("mrkdwn", line, false, false)
		} else if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
			// Bullet point
			bullet := strings.TrimLeft(line, "-*")
			textObj = slack.NewTextBlockObject("mrkdwn", "• "+bullet, false, false)
		} else {
			// Regular text
			textObj = slack.NewTextBlockObject("mrkdwn", line, false, false)
		}

		section := slack.NewSectionBlock(textObj, nil, nil)
		blocks = append(blocks, section)
	}
	return blocks
}

// GetConversationHistory fetches the conversation history from a channel.
func (c *Client) GetConversationHistory(channelID string, start, end time.Time) ([]slack.Message, error) {
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

	// Reverse the messages to be in chronological order
	for i, j := 0, len(history.Messages)-1; i < j; i, j = i+1, j-1 {
		history.Messages[i], history.Messages[j] = history.Messages[j], history.Messages[i]
	}

	return history.Messages, nil
}

// GetUserName fetches a user's name from the cache or the API.
func (c *Client) GetUserName(userID string) string {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	if userName, ok := c.userCache[userID]; ok {
		return userName
	}

	user, err := c.api.GetUserInfo(userID)
	if err != nil {
		log.Printf("Error getting user info for %s: %v", userID, err)
		return userID // Fallback to user ID
	}

	c.userCache[userID] = user.Name
	return user.Name
}

// GetChannelName fetches a channel's name from the cache or the API.
func (c *Client) GetChannelName(channelID string) string {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	if channelName, ok := c.channelCache[channelID]; ok {
		return channelName
	}

	channel, err := c.api.GetConversationInfo(&slack.GetConversationInfoInput{ChannelID: channelID})
	if err != nil {
		log.Printf("Error getting channel info for %s: %v", channelID, err)
		return channelID // Fallback to channel ID
	}

	c.channelCache[channelID] = channel.Name
	return channel.Name
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

