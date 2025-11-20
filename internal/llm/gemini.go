package llm

import (
	"context"
	"log"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GenerateContent is a simple function that takes an API key and a prompt,
// and returns the generated content from the Gemini API.
func GenerateContent(ctx context.Context, apiKey, prompt string) (string, error) {
	if apiKey == "YOUR_GEMINI_API_KEY_HERE" || apiKey == "" {
		return "AI service is not configured. Please add your Gemini API key to config.yaml.", nil
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		// Log the error but return a user-friendly message
		log.Printf("Failed to create Gemini client: %v", err)
		return "Sorry, there was an issue connecting to the AI service.", err
	}
	defer client.Close()

	log.Println("---------------------------------")
	log.Printf("Sending prompt to Gemini:\n%s", prompt)
	log.Println("---------------------------------")


	model := client.GenerativeModel("gemini-pro-latest") // Using a known stable model
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Printf("Failed to generate content: %v", err)
		return "Sorry, I had trouble generating a response.", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "I don't have a response for that.", nil
	}

	var responseText string
	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			if txt, ok := part.(genai.Text); ok {
				responseText += string(txt)
			}
		}
	}

	log.Println("---------------------------------")
	log.Printf("Received response from Gemini:\n%s", responseText)
	log.Println("---------------------------------")


	return responseText, nil
}
