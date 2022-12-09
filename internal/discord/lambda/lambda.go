package lambda

import (
	"bytes"
	crypto "crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/bwmarrin/discordgo"

	"game-server/internal/discord/bot"
	"game-server/pkg/aws/instance"
	"game-server/pkg/aws/sqs"
	"game-server/pkg/discord"
	customError "game-server/pkg/errors"
)

const (
	EnvPublicKey  = "PUBLIC_KEY"
	EnvInstanceId = "INSTANCE_ID"
	EnvSqsUrl     = "MESSAGE_QUEUE_URL"
)

var (
	unauthorizedResponse = events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusUnauthorized,
		Body:       "Invalid request signature",
	}

	badRequestResponse = events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusBadRequest,
		Body:       "Invalid request",
	}

	internalErrorResponse = events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       "Internal server error",
	}
)

type Handler struct {
	logger *log.Logger

	// Env variables
	publicKey  crypto.PublicKey
	instanceId string
	sqsUrl     string

	httpClient *http.Client

	// AWS
	instanceClient instance.ClientIFace
	sqsClient      sqs.ClientIFace
}

func New() *Handler {
	// Configure HTTP client
	httpClient := http.DefaultClient
	httpClient.Timeout = 3 * time.Second

	return &Handler{
		logger:         log.Default(),
		httpClient:     httpClient,
		instanceClient: instance.New(),
		sqsClient:      sqs.New(),
	}
}

func (h *Handler) Handle(event events.APIGatewayV2HTTPRequest) events.APIGatewayV2HTTPResponse {
	if err := h.loadEnv(); err != nil {
		h.logger.Print(err)
		return internalErrorResponse
	}

	rsp := h.handle(event)

	// Set content-type header in response
	if rsp.Headers == nil {
		rsp.Headers = make(map[string]string, 1)
	}
	rsp.Headers["Content-Type"] = "application/json"

	return rsp
}

func (h *Handler) handle(event events.APIGatewayV2HTTPRequest) events.APIGatewayV2HTTPResponse {
	// Verify request signature
	eventBody := []byte(event.Body)
	timestamp := event.Headers[discord.TimestampHeader]
	signature := event.Headers[discord.SignatureHeader]
	if !discord.Authenticate(eventBody, timestamp, signature, h.publicKey) {
		return unauthorizedResponse
	}

	// Parse request from event body
	var req discordgo.Interaction
	if err := json.Unmarshal(eventBody, &req); err != nil {
		return badRequestResponse
	}

	// Acknowledge a ping
	if req.Type == discordgo.InteractionPing {
		rsp := discordgo.InteractionResponse{
			Type: discordgo.InteractionResponsePong,
		}
		body, _ := json.Marshal(rsp)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       string(body),
		}
	}

	// Connect to AWS instance
	if err := h.instanceClient.Connect(); err != nil {
		h.logger.Print(err)
		return internalErrorResponse
	}

	// Get instance state
	state, err := h.instanceClient.GetInstanceState(h.instanceId)
	if err != nil {
		h.logger.Print(err)
		return internalErrorResponse
	}

	switch state {
	// Forward request to server when it's already running
	case instance.InstanceRunningState:
		return h.forwardToInstance(eventBody, event.Headers)

	// Send error message when server is already starting up
	case instance.InstancePendingState:
		rsp := discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Server is still starting up!",
			},
		}
		body, _ := json.Marshal(rsp)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       string(body),
		}

	// Start server and send deferred response when it's not already running or starting up
	default:
		// Attempt to start
		if err := h.instanceClient.StartInstance(h.instanceId); err != nil {
			h.logger.Print(err)
			return internalErrorResponse
		}

		// Forward request to deferred message queue
		h.sqsClient.ConnectWithSession(h.instanceClient.GetSession())
		if err := h.sqsClient.Send(h.sqsUrl, event.Body); err != nil {
			h.logger.Print(err)
			return internalErrorResponse
		}

		// Return deferred response
		rsp := discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}
		body, _ := json.Marshal(rsp)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       string(body),
		}
	}
}

func (h *Handler) loadEnv() error {
	// Get expected env variables
	publicKey := os.Getenv(EnvPublicKey)
	h.instanceId = os.Getenv(EnvInstanceId)
	h.sqsUrl = os.Getenv(EnvSqsUrl)
	if publicKey == "" || h.instanceId == "" || h.sqsUrl == "" {
		return customError.MissingEnvErr{EnvMap: map[string]string{
			EnvPublicKey:  publicKey,
			EnvInstanceId: h.instanceId,
			EnvSqsUrl:     h.sqsUrl,
		}}
	}

	// Decode public key
	var err error
	if h.publicKey, err = discord.DecodePublicKey(publicKey); err != nil {
		return fmt.Errorf("invalid public key")
	}

	return nil
}

func (h *Handler) forwardToInstance(reqBody []byte, headers map[string]string) events.APIGatewayV2HTTPResponse {
	// Get endpoint
	instanceAddress, err := h.instanceClient.GetInstanceAddress(h.instanceId)
	if err != nil {
		h.logger.Print(err)
		return internalErrorResponse
	}
	endpoint := fmt.Sprintf("http://%s%s", instanceAddress, bot.BotEndpoint)

	// Build HTTP request
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		h.logger.Print(err)
		return internalErrorResponse
	}
	req.Header.Set(discord.SignatureHeader, headers[discord.SignatureHeader])
	req.Header.Set(discord.TimestampHeader, headers[discord.TimestampHeader])

	// Make HTTP call
	rsp, err := h.httpClient.Do(req)
	if err != nil {
		h.logger.Print(err)
		return internalErrorResponse
	}

	// Convert response body
	rspBody := new(strings.Builder)
	if _, err := io.Copy(rspBody, rsp.Body); err != nil {
		h.logger.Print(err)
		return internalErrorResponse
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: rsp.StatusCode,
		Body:       rspBody.String(),
	}
}
