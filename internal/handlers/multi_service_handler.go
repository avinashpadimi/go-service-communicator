package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gemini/go-service-communicator/internal/services"
)

// MultiServiceHandler handles requests for multiple services.
type MultiServiceHandler struct {
	services map[string]services.Communicator
}

// NewMultiServiceHandler creates a new MultiServiceHandler.
func NewMultiServiceHandler(services map[string]services.Communicator) *MultiServiceHandler {
	return &MultiServiceHandler{
		services: services,
	}
}

// SendMessageHandler handles requests to send a message to a specified service.
func (h *MultiServiceHandler) SendMessageHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service     string `json:"service"`
		Destination string `json:"destination"`
		Message     string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	service, ok := h.services[req.Service]
	if !ok {
		http.Error(w, fmt.Sprintf("service not found: %s", req.Service), http.StatusBadRequest)
		return
	}

	if err := service.SendMessage(req.Destination, req.Message); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "message sent"})
}
