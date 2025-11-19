package handlers

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gemini/go-service-communicator/internal/agent"
	"github.com/gemini/go-service-communicator/internal/services/jira"
	slackclient "github.com/gemini/go-service-communicator/internal/services/slack"
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
		go h.processSummaryCommand(s.ChannelID, s.Text)

	default:
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Unsupported command"))
	}
}

func (h *SlashCommandHandler) processSummaryCommand(requestChannelID, commandText string) {
	duration := 24 * time.Hour // Default to 24 hours
	var err error

	commandText = strings.TrimSpace(commandText)
	if commandText != "" {
		duration, err = parseDuration(commandText)
		if err != nil {
			h.slackClient.SendMessage(requestChannelID, fmt.Sprintf("Error: Invalid time range format. Please use a format like '48h' or '7d'. Using default of 24h."))
			duration = 24 * time.Hour // Fallback to default
		}
	}

	endTime := time.Now()
	startTime := endTime.Add(-duration)
	jiraQuery := "status=new"

	publicChannels, err := h.slackClient.GetPublicChannels()
	if err != nil {
		// Log the error, and optionally send an error message to the user.
		h.slackClient.SendMessage(requestChannelID, "Error: Could not fetch public channels.")
		return
	}
	fmt.Println("public channels:", publicChannels)

	var allMessages []string
	for _, channelID := range publicChannels {
		messages, err := h.slackClient.GetConversationHistory(channelID, startTime, endTime)
		if err != nil {
			// Log the error, maybe skip this channel and continue.
			continue
		}
		allMessages = append(allMessages, messages...)
	}

	jiraIssues, err := h.jiraClient.FetchIssues(jiraQuery)
	if err != nil {
		// Log the error.
		h.slackClient.SendMessage(requestChannelID, "Error: Could not fetch Jira issues.")
		return
	}

	summary := h.agent.ConsolidateInfo(allMessages, jiraIssues)

	h.slackClient.SendMessage(requestChannelID, summary)
}

// parseDuration parses a string like "7d" or "24h" into a time.Duration.
func parseDuration(durationStr string) (time.Duration, error) {
	re := regexp.MustCompile(`^(\d+)([dh])`)
	matches := re.FindStringSubmatch(durationStr)

	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid duration format: %s", durationStr)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err // Should not happen with the regex
	}

	unit := matches[2]
	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown time unit: %s", unit)
	}
}
