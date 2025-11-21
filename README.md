# Go Service Communicator

This project is a boilerplate for a Go application that communicates with multiple services like Slack, Jira, and GitHub. It is designed to be modular and easily extensible.

## Prerequisites

- Go 1.21 or higher
- Docker

## Getting Started

### Local Development

1.  **Clone the repository:**
    ```sh
    git clone https://github.com/gemini/go-service-communicator.git
    cd go-service-communicator
    ```

2.  **Install dependencies:**
    ```sh
    go mod tidy
    ```

3.  **Configure your services:**
    Open `config.yaml` and add your service tokens/keys.
    ```yaml
    slack:
      token: "your-slack-bot-token"
      signing_secret: "your-slack-signing-secret"
    ```

4.  **Run the application:**
    ```sh
    go run cmd/server/main.go
    ```
    The server will start on `http://localhost:8080`.

### Docker

1.  **Build the Docker image:**
    ```sh
    docker build -t go-service-communicator .
    ```

2.  **Run the Docker container:**
    ```sh
    docker run -p 8080:8080 go-service-communicator
    ```

## API Usage

### Send a Message

Send a `POST` request to `/send` with a JSON body. The `service` field determines which service to use.

#### Slack Example

```sh
curl -X POST http://localhost:8080/send \
-H "Content-Type: application/json" \
-d '{
    "service": "slack",
    "destination": "#general",
    "message": "Hello, from the Go Service Communicator!"
}'
```

#### Jira Example

```sh
curl -X POST http://localhost:8080/send \
-H "Content-Type: application/json" \
-d '{
    "service": "jira",
    "destination": "PROJ-123",
    "message": "This is a comment for the Jira issue."
}'
```

## Slack App Configuration

To enable all features of this application, you need to grant the following permissions (scopes) to your bot token in your Slack App settings under "OAuth & Permissions":

- `channels:history`: View messages in public channels that your app has been added to.
- `channels:read`: View basic information about public channels in a workspace.
- `chat:write`: Send messages as your app.
- `commands`: Add shortcuts and/or slash commands that people can use.
- `app_mentions:read`: Read messages that directly mention your app in conversations.
- `users:read`: View people in a workspace.

### Event Subscriptions

To receive events from Slack, you need to configure your Slack App:

1.  **Enable Event Subscriptions:** In your Slack App settings, go to "Event Subscriptions" and turn it on.
2.  **Request URL:** Set the Request URL to `http://<your-public-url>/slack/events`. When you enter this URL, Slack will send a challenge to your running application to verify the URL.
3.  **Subscribe to Bot Events:** Under "Subscribe to bot events", add the `app_mention` event. This will send an event to your application whenever your bot is mentioned in a channel.
4.  **Reinstall App:** Reinstall your app to the workspace to apply the new permissions.

Your bot should now respond to @mentions in any channel it's a member of.

### Slash Commands

To create a slash command that triggers the summary generation:

1.  **Go to Slash Commands:** In your Slack App settings, go to "Slash Commands".
2.  **Create New Command:** Click "Create New Command".
3.  **Command:** Enter `/summary`.
4.  **Request URL:** Set the Request URL to `http://<your-public-url>/slack/command`.
5.  **Short Description:** Enter a short description, e.g., "Generates a summary of recent activity".
6.  **Save:** Save the command and reinstall your app to the workspace.

You can now run `/summary` in any channel the bot is in to get a summary of the last 24 hours of conversation and new Jira issues.
