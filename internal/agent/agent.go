package agent

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gemini/go-service-communicator/internal/llm"
	"github.com/gemini/go-service-communicator/internal/services/slack"
)

// SummaryContext holds information about the last summary generated for a user.
type SummaryContext struct {
	Summary   string
	ChannelID string
}

// Processor is the agent that handles business logic.
type Processor struct {
	apiKey       string
	slackClient  *slack.Client
	lastSummary  map[string]SummaryContext
	summaryMutex sync.Mutex
}

// New creates a new Processor.
func New(apiKey string, slackClient *slack.Client) *Processor {
	return &Processor{
		apiKey:      apiKey,
		slackClient: slackClient,
		lastSummary: make(map[string]SummaryContext),
	}
}

// SetLastSummary stores the most recent summary generated for a user.
func (p *Processor) SetLastSummary(userID, channelID, summary string) {
	p.summaryMutex.Lock()
	defer p.summaryMutex.Unlock()
	log.Printf("Storing summary context for user %s in channel %s", userID, channelID)
	p.lastSummary[userID] = SummaryContext{Summary: summary, ChannelID: channelID}
}

// ProcessMessage is for simple, non-contextual AI responses (e.g., for @mentions).
func (p *Processor) ProcessMessage(userID, channelID, message string) string {
	lowerMessage := strings.ToLower(message)
	if strings.Contains(lowerMessage, "summary") || strings.Contains(lowerMessage, "summarize") {
		return p.performSummary(userID, message, channelID)
	}

	prompt := fmt.Sprintf(`A user mentioned the bot with the following message. Please provide a helpful response in Slack's Block Kit JSON format. The JSON should be a valid array of blocks.

Example of a simple response:
[
  {
    "type": "section",
    "text": {
      "type": "mrkdwn",
      "text": "This is a simple message."
    }
  }
]

User message: "%s"`, message)
	response, err := llm.GenerateContent(context.Background(), p.apiKey, prompt)
	if err != nil {
		return response // Error message is already formatted
	}
	return cleanGeminiResponse(response)
}

// ProcessDM is for conversational AI responses in direct messages.
func (p *Processor) ProcessDM(userID string, history []string, latestMessage string) string {
	var builder strings.Builder
	builder.WriteString(`You are a helpful and friendly conversational AI assistant. Continue the following conversation naturally.
Please provide a response in Slack's Block Kit JSON format. The JSON should be a valid array of blocks.

Example of a simple response:
[
  {
    "type": "section",
    "text": {
      "type": "mrkdwn",
      "text": "This is a simple message."
    }
  }
]

`)

	// Check for specific intents
	lowerMessage := strings.ToLower(latestMessage)
	if strings.Contains(lowerMessage, "summary") || strings.Contains(lowerMessage, "summarize") {
		return p.performSummary(userID, latestMessage, "") // Pass empty channelID
	}
	if strings.Contains(lowerMessage, "mentions") || strings.Contains(lowerMessage, "tagged") || strings.Contains(lowerMessage, "missed") {
		return p.findUserMentions(userID)
	}


	// Check if there's a recent summary to add as context.
	p.summaryMutex.Lock()
	if summaryCtx, ok := p.lastSummary[userID]; ok {
		log.Printf("Found summary context for user %s", userID)
		builder.WriteString("CONTEXT: The user was just shown the following summary. Use this summary to answer any follow-up questions.\n--- SUMMARY START ---\n")
		builder.WriteString(summaryCtx.Summary)
		if summaryCtx.ChannelID != "" {
			builder.WriteString(fmt.Sprintf("\n(The summary was for channel %s)", summaryCtx.ChannelID))
		}
		builder.WriteString("\n--- SUMMARY END ---\n\n")
		// The summary context is now loaded. Delete it so it's not used in the *next* turn.
		delete(p.lastSummary, userID)
	}
	p.summaryMutex.Unlock()

	builder.WriteString("--- CONVERSATION HISTORY ---\n")
	for _, msg := range history {
		builder.WriteString(msg + "\n")
	}
	builder.WriteString("User: " + latestMessage + "\n")
	builder.WriteString("--- END HISTORY ---\n\n")
	builder.WriteString("Assistant (in JSON format):")

	prompt := builder.String()

	response, err := llm.GenerateContent(context.Background(), p.apiKey, prompt)
	if err != nil {
		return response // Error message is already formatted
	}
	return cleanGeminiResponse(response)
}

// performSummary fetches channel history and generates a summary.
func (p *Processor) performSummary(userID, message, channelID string) string {
	// Default to 1 day if parsing fails
	duration := 24 * time.Hour
	// Try to parse a duration from the message (e.g., "10 days")
	re := regexp.MustCompile(`(\d+)\s*d`)
	matches := re.FindStringSubmatch(message)
	if len(matches) == 2 {
		days, err := strconv.Atoi(matches[1])
		if err == nil {
			duration = time.Duration(days) * 24 * time.Hour
		}
	}

	endTime := time.Now()
	startTime := endTime.Add(-duration)

	var channelsToSummarize []string
	if channelID != "" {
		channelsToSummarize = []string{channelID}
	} else {
		publicChannels, err := p.slackClient.GetPublicChannels()
		if err != nil {
			log.Printf("Error fetching public channels: %v", err)
			return "Sorry, I couldn't fetch the list of public channels."
		}
		channelsToSummarize = publicChannels
	}

	var allMessages []string
	for _, chID := range channelsToSummarize {
		messages, err := p.slackClient.GetConversationHistory(chID, startTime, endTime)
		if err != nil {
			log.Printf("Error fetching history for channel %s: %v", chID, err)
			continue // Skip channels we can't access
		}
		// Highlight mentions of the user in the messages before sending to AI
		for i, msg := range messages {
			messages[i] = highlightMentions(msg, userID)
		}
		allMessages = append(allMessages, messages...)
	}

	if len(allMessages) == 0 {
		return "I couldn't find any messages in the specified time period."
	}

	// Create a prompt for the AI to summarize
	var promptBuilder strings.Builder
	promptBuilder.WriteString(`Please provide a concise summary of the following Slack messages in Slack's Block Kit JSON format.

Example of the desired format:
[
    {
        "type": "header",
        "text": {
            "type": "plain_text",
            "text": "Summary of Public Channels"
        }
    },
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "Here is a summary of the recent conversations."
        }
    },
    {
        "type": "divider"
    }
]

Slack Messages:
`)
	for _, msg := range allMessages {
		promptBuilder.WriteString("- " + msg + "\n")
	}

	summary, err := llm.GenerateContent(context.Background(), p.apiKey, promptBuilder.String())
	if err != nil {
		return "I was able to fetch the messages, but I encountered an error while generating the summary."
	}

	cleanSummary := cleanGeminiResponse(summary)
	p.SetLastSummary(userID, channelID, cleanSummary)
	return cleanSummary
}

// findUserMentions searches for messages where the given userID was mentioned.
func (p *Processor) findUserMentions(userID string) string {
	query := fmt.Sprintf("<@%s>", userID)
	searchResult, err := p.slackClient.SearchMessages(query)
	if err != nil {
		log.Printf("Error searching for mentions for user %s: %v", userID, err)
		if strings.Contains(err.Error(), "not_allowed_token_type") { // Specific error for user token issue
			return "I can't search for your mentions because I'm missing the `search:read` permission or the token type is not allowed. Please ensure I have the `search:read` scope and that your workspace allows bot tokens for search."
		}
		if strings.Contains(err.Error(), "missing_scope") {
			return "I can't search for your mentions because I'm missing the `search:read` permission. Please add it to my Slack App configuration."
		}
		return "Sorry, I couldn't search for your mentions."
	}

	if searchResult == nil || len(searchResult.Matches) == 0 {
		return "I couldn't find any recent mentions of you."
	}

	var builder strings.Builder
	builder.WriteString("Here are some recent mentions of you:\n\n")
	for i, match := range searchResult.Matches {
		if i >= 5 { // Limit to top 5 mentions for brevity
			builder.WriteString(fmt.Sprintf("\n...and %d more. Ask me to summarize if you want to know more!", len(searchResult.Matches)-5))
			break
		}
		// Highlight the user's mention in the search result
		highlightedText := highlightMentions(match.Text, userID)
		builder.WriteString(fmt.Sprintf("- In #%s, <@%s> said: \"%s\"\n", match.Channel.Name, match.User, highlightedText))
	}

	return builder.String()
}

// continueConversation handles a regular conversational turn.
func (p *Processor) continueConversation(userID string, history []string) string {
	var builder strings.Builder
	builder.WriteString("You are a helpful and friendly conversational AI assistant. Continue the following conversation naturally.\n\n")

	// Check if there's a recent summary to add as context.
	p.summaryMutex.Lock()
	if summaryCtx, ok := p.lastSummary[userID]; ok {
		log.Printf("Found summary context for user %s", userID)
		builder.WriteString("CONTEXT: The user was just shown the following summary. Use this summary to answer any follow-up questions.\n--- SUMMARY START ---\n")
		builder.WriteString(summaryCtx.Summary)
		if summaryCtx.ChannelID != "" {
			builder.WriteString(fmt.Sprintf("\n(The summary was for channel %s)", summaryCtx.ChannelID))
		}
		builder.WriteString("\n--- SUMMARY END ---\n\n")
		// The summary context is now loaded. Delete it so it's not used in the *next* turn.
		delete(p.lastSummary, userID)
	}
	p.summaryMutex.Unlock()

	builder.WriteString("--- CONVERSATION HISTORY ---\n")
	for _, msg := range history {
		builder.WriteString(msg + "\n")
	}
	// Add the latest message from the user to the history for the AI
	builder.WriteString("--- END HISTORY ---\n\n")
	builder.WriteString("Assistant:")

	prompt := builder.String()

	response, err := llm.GenerateContent(context.Background(), p.apiKey, prompt)
	if err != nil {
		return response // Error message is already formatted
	}
	return cleanGeminiResponse(response)
}

// ConsolidateInfo uses the AI to create a summary from Slack messages and Jira issues.
// This is used by the /summary slash command.
func (p *Processor) ConsolidateInfo(userID string, slackMessages, jiraIssues []string) string { // Added userID
	var builder strings.Builder
	builder.WriteString(`Please provide a concise summary of the following activities in Slack's Block Kit JSON format. The JSON should be a valid array of blocks.

Use a header for "Slack Conversations" and "Jira Issues", and a divider between them.

Example of the desired format:
[
    {
        "type": "header",
        "text": {
            "type": "plain_text",
            "text": "Activity Summary"
        }
    },
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "*Slack Conversations:*"
        }
    },
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "- Message 1"
        }
    },
    {
        "type": "divider"
    },
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "*Jira Issues:*"
        }
    },
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "- Issue 1"
        }
    }
]

`)

	if len(slackMessages) > 0 {
		builder.WriteString("Slack Conversations:\n")
		for _, msg := range slackMessages {
			builder.WriteString(fmt.Sprintf("- %s\n", highlightMentions(msg, userID))) // Highlight mentions
		}
	}

	if len(jiraIssues) > 0 {
		builder.WriteString("\nJira Issues:\n")
		for _, issue := range jiraIssues {
			builder.WriteString(fmt.Sprintf("- %s\n", issue))
		}
	}

	if len(slackMessages) == 0 && len(jiraIssues) == 0 {
		return "There were no activities to summarize in the given time period."
	}

	prompt := builder.String()

	summary, err := llm.GenerateContent(context.Background(), p.apiKey, prompt)
	if err != nil {
		return "I was able to fetch the activities, but I encountered an error while generating the summary."
	}
	return cleanGeminiResponse(summary)
}

// highlightMentions replaces mentions of the userID with a bolded version for Slack markdown.
func highlightMentions(text, userID string) string {
	mentionTag := fmt.Sprintf("<@%s>", userID)
	highlightedMentionTag := fmt.Sprintf("*<@%s>*", userID)
	return strings.ReplaceAll(text, mentionTag, highlightedMentionTag)
}

// cleanGeminiResponse removes markdown formatting from the Gemini response.
func cleanGeminiResponse(response string) string {
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)
	return response
}
