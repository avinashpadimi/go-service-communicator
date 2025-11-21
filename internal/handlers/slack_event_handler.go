package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gemini/go-service-communicator/internal/agent"
	"github.com/gemini/go-service-communicator/internal/services/slack"
	"github.com/slack-go/slack/slackevents"
)

const maxHistory = 10

// SlackEventHandler handles Slack event subscriptions.
type SlackEventHandler struct {
	slackClient         *slack.Client
	agent               *agent.Processor
	botUserID           string
	conversationHistory map[string][]string
}

// NewSlackEventHandler creates a new SlackEventHandler.
func NewSlackEventHandler(slackClient *slack.Client, agent *agent.Processor, botUserID string) *SlackEventHandler {
	return &SlackEventHandler{
		slackClient:         slackClient,
		agent:               agent,
		botUserID:           botUserID,
		conversationHistory: make(map[string][]string),
	}
}

// HandleEvent handles incoming Slack events.
func (h *SlackEventHandler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal(body, &r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(r.Challenge))
		return
	}

	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		// Acknowledge the event immediately to prevent Slack from retrying.
		w.WriteHeader(http.StatusOK)

		// Run the actual processing in a goroutine.
		go func() {
			innerEvent := eventsAPIEvent.InnerEvent
			switch ev := innerEvent.Data.(type) {
			case *slackevents.AppMentionEvent:
				// Ignore messages from the bot itself
				if ev.User == h.botUserID {
					return
				}
				// For mentions, we don't use history, just a direct response.
				response := h.agent.ProcessMessage(ev.User, ev.Channel, ev.Text)
				h.slackClient.SendMessage(ev.Channel, response)

			case *slackevents.MessageEvent:
				// Handle direct messages to the bot
				if ev.ChannelType == "im" {
					// Ignore messages from the bot itself to prevent loops
					if ev.User == h.botUserID {
						return
					}

					// Retrieve conversation history
					history := h.conversationHistory[ev.User]

					// Get the AI's response
					response := h.agent.ProcessDM(ev.User, history, ev.Text)

					// Update history with the new turn
					history = append(history, "User: "+ev.Text)
					history = append(history, "Assistant: "+response)

					// Trim history to keep it from growing indefinitely
					if len(history) > maxHistory {
						history = history[len(history)-maxHistory:]
					}
					h.conversationHistory[ev.User] = history

					h.slackClient.SendMessage(ev.Channel, response)
				}
			}
		}()
	}
}
