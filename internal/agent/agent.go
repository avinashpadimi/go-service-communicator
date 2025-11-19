package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/gemini/go-service-communicator/internal/llm"
)

// Processor is a simple agent that processes a message.
type Processor struct {
	apiKey string
}

// New creates a new Processor.
func New(apiKey string) *Processor {
	return &Processor{apiKey: apiKey}
}

// ProcessMessage sends the message to the AI and returns the response.
func (p *Processor) ProcessMessage(message string) string {
	response, err := llm.GenerateContent(context.Background(), p.apiKey, message)
	if err != nil {
		// The error is already logged in the llm package, just return the user-facing message.
		return response
	}
	return response
}

// ConsolidateInfo uses the AI to create a summary from Slack messages and Jira issues.
func (p *Processor) ConsolidateInfo(slackMessages, jiraIssues []string) string {
	var builder strings.Builder
	builder.WriteString("Please summarize the following activities.\n\n")
	builder.WriteString("Slack Conversations:\n")
	for _, msg := range slackMessages {
		builder.WriteString(fmt.Sprintf("- %s\n", msg))
	}
	builder.WriteString("\nJira Issues:\n")
	for _, issue := range jiraIssues {
		builder.WriteString(fmt.Sprintf("- %s\n", issue))
	}

	prompt := builder.String()

	summary, err := llm.GenerateContent(context.Background(), p.apiKey, prompt)
	if err != nil {
		// Fallback to the old simple summary

		log.Printf("AI summary failed, falling back to simple summary. Error: %v", err)
		return p.simpleSummary(slackMessages, jiraIssues)
	}
	return summary
}

func (p *Processor) simpleSummary(slackMessages, jiraIssues []string) string {
	var builder strings.Builder
	builder.WriteString("Summary of activities:\n\n")
	builder.WriteString(fmt.Sprintf("*Slack Conversations (%d messages):*\n", len(slackMessages)))
	for _, msg := range slackMessages {
		builder.WriteString(fmt.Sprintf("- %s\n", msg))
	}
	builder.WriteString(fmt.Sprintf("\n*Jira Issues (%d issues):*\n", len(jiraIssues)))
	for _, issue := range jiraIssues {
		builder.WriteString(fmt.Sprintf("- %s\n", issue))
	}
	return builder.String()
}
