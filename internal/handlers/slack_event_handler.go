package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gemini/go-service-communicator/internal/agent"
	"github.com/gemini/go-service-communicator/internal/services/slack"
	"github.com/slack-go/slack/slackevents"
)

// SlackEventHandler handles Slack event subscriptions.
type SlackEventHandler struct {
	slackClient *slack.Client
	agent       *agent.Processor
}

// NewSlackEventHandler creates a new SlackEventHandler.
func NewSlackEventHandler(slackClient *slack.Client, agent *agent.Processor) *SlackEventHandler {
	return &SlackEventHandler{
		slackClient: slackClient,
		agent:       agent,
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
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			// Don't respond to messages from the bot itself.
			if ev.User == "" {
				return
			}
			// Get the processed response from the agent.
			response := h.agent.ProcessMessage(ev.Text)
			// Send the response back to the channel.
			h.slackClient.SendMessage(ev.Channel, response)
		}
	}
}
