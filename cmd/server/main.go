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
	agentProcessor := agent.New(cfg.Gemini.APIKey)

	// Create a map of services
	communicators := map[string]services.Communicator{
		"slack": slackClient,
		"jira":  jiraClient,
	}

	// Initialize handlers
	multiServiceHandler := handlers.NewMultiServiceHandler(communicators)
	slackEventHandler := handlers.NewSlackEventHandler(slackClient, agentProcessor)
	slashCommandHandler := handlers.NewSlashCommandHandler(slackClient, jiraClient, agentProcessor, cfg.Slack.SigningSecret)

	// Create router
	r := mux.NewRouter()

	// Register routes
	r.HandleFunc("/send", multiServiceHandler.SendMessageHandler).Methods("POST")
	r.HandleFunc("/slack/events", slackEventHandler.HandleEvent).Methods("POST")
	r.HandleFunc("/slack/command", slashCommandHandler.HandleCommand).Methods("POST")

	// Start server
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8081", r); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
