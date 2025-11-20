package main

import (
	"log"
	"net/http"

	"github.com/gemini/go-service-communicator/internal/agent"
	"github.com/gemini/go-service-communicator/internal/config"
	"github.com/gemini/go-service-communicator/internal/handlers"
	"github.com/gemini/go-service-communicator/internal/services"
	"github.com/gemini/go-service-communicator/internal/services/jira"
	"github.com/gemini/go-service-communicator/internal/services/slack"
	"github.com/gorilla/mux"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	// Initialize services
	slackClient := slack.New(cfg.Slack.Token)
	jiraClient := jira.New()
	agentProcessor := agent.New(cfg.Gemini.APIKey, slackClient)

	// Get bot's own user ID to prevent loops
	authTest, err := slackClient.AuthTest()
	if err != nil {
		log.Fatalf("could not authenticate with Slack: %v", err)
	}
	botUserID := authTest.UserID

	// Create a map of services
	communicators := map[string]services.Communicator{
		"slack": slackClient,
		"jira":  jiraClient,
	}

	// Initialize handlers
	multiServiceHandler := handlers.NewMultiServiceHandler(communicators)
	slackEventHandler := handlers.NewSlackEventHandler(slackClient, agentProcessor, botUserID)
	slashCommandHandler := handlers.NewSlashCommandHandler(slackClient, jiraClient, agentProcessor, cfg.Slack.SigningSecret)

	// Create router
	r := mux.NewRouter()

	// Register routes
	r.HandleFunc("/send", multiServiceHandler.SendMessageHandler).Methods("POST")
	r.HandleFunc("/slack/events", slackEventHandler.HandleEvent).Methods("POST")
	r.HandleFunc("/slack/command", slashCommandHandler.HandleCommand).Methods("POST")

	// Start server
	log.Println("Starting server on :8082")
	if err := http.ListenAndServe(":8082", r); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
