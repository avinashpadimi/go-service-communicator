package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gemini/go-service-communicator/internal/agent"
	"github.com/gemini/go-service-communicator/internal/services/jira"
	slackclient "github.com/gemini/go-service-communicator/internal/services/slack"
	"github.com/gemini/go-service-communicator/internal/util"
	"github.com/slack-go/slack"
)

// SlashCommandHandler handles slash command requests from Slack.
type SlashCommandHandler struct {
	slackClient   *slackclient.Client
	jiraClient    *jira.Client
	agent         *agent.Processor
	signingSecret string
}

// NewSlashCommandHandler creates a new SlashCommandHandler.
func NewSlashCommandHandler(slackClient *slackclient.Client, jiraClient *jira.Client, agent *agent.Processor, signingSecret string) *SlashCommandHandler {
	return &SlashCommandHandler{
		slackClient:   slackClient,
		jiraClient:    jiraClient,
		agent:         agent,
		signingSecret: signingSecret,
	}
}

// HandleCommand handles the slash command.
func (h *SlashCommandHandler) HandleCommand(w http.ResponseWriter, r *http.Request) {
	verifier, err := slack.NewSecretsVerifier(r.Header, h.signingSecret)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	r.Body = io.NopCloser(io.TeeReader(r.Body, &verifier))
	s, err := slack.SlashCommandParse(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err = verifier.Ensure(); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch s.Command {
	case "/summary":
		// Acknowledge the command immediately to avoid timeouts.
		w.WriteHeader(http.StatusOK)

		// Run the actual logic in a goroutine to avoid blocking.
		go h.processSummaryCommand(s.UserID, s.ChannelID, s.Text)

	default:
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Unsupported command"))
	}
}

func (h *SlashCommandHandler) processSummaryCommand(userID, requestChannelID, commandText string) {
	duration := 24 * time.Hour // Default to 24 hours
	var err error

	commandText = strings.TrimSpace(commandText)
	if commandText != "" {
		duration, err = util.ParseDuration(commandText)
		if err != nil {
			h.slackClient.SendMessage(requestChannelID, fmt.Sprintf("Error: Invalid time range format. Please use a format like '1h', '7d', '2m', or '1y'. Using default of 24h."))
			duration = 24 * time.Hour // Fallback to default
		}
	}

	endTime := time.Now()
	startTime := endTime.Add(-duration)
	jiraQuery := "status=new"

	rawMessages, err := h.slackClient.GetConversationHistory(requestChannelID, startTime, endTime)
	if err != nil {
		// Log the error, and optionally send an error message to the user.
		h.slackClient.SendMessage(requestChannelID, "Error: Could not fetch message history for this channel. Make sure I have been invited by using '/invite @<bot-name>'.")
		return
	}
	for i := range rawMessages {
		rawMessages[i].Channel = requestChannelID
	}

	jiraIssues, err := h.jiraClient.FetchIssues(jiraQuery)
	if err != nil {
		// Log the error.
		h.slackClient.SendMessage(requestChannelID, "Error: Could not fetch Jira issues.")
		return
	}

	summary := h.agent.ConsolidateInfo(userID, rawMessages, jiraIssues)

	// Store the summary for potential follow-up questions in a DM.
	h.agent.SetLastSummary(userID, requestChannelID, summary, rawMessages)

	h.slackClient.SendMessage(requestChannelID, summary)
}
