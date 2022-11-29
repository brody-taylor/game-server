package discordlambda

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

	"game-server/internal/discordbot"
	"game-server/pkg/aws/instance"
)

const (
	EnvInstanceId = "INSTANCE_ID"
	EnvPublicKey  = "PUBLIC_KEY"
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

type MissingEnvErr struct {
	envMap map[string]string
}

func (e MissingEnvErr) Error() string {
	// Get keys of missing environment variables
	missingKeys := make([]string, 0, len(e.envMap))
	for key, val := range e.envMap {
		if val == "" {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		allKeys := strings.Join(missingKeys, ", ")
		return fmt.Sprintf("insufficient env variables: [%s]", allKeys)
	}
	return "insufficient env variables"
}

type Handler struct {
	logger *log.Logger

	// Env variables
	publicKey  crypto.PublicKey
	instanceId string

	httpClient *http.Client

	// AWS
	instanceClient instance.ClientIFace
}

func New() *Handler {
	// Configure HTTP client
	httpClient := http.DefaultClient
	httpClient.Timeout = 3 * time.Second

	return &Handler{
		logger:         log.Default(),
		httpClient:     httpClient,
		instanceClient: instance.New(),
	}
}

func (h *Handler) Handle(event events.APIGatewayV2HTTPRequest) events.APIGatewayV2HTTPResponse {
	if err := h.loadEnv(); err != nil {
		h.logger.Print(err)
		return internalErrorResponse
	}

	// Verify request signature
	eventBody := []byte(event.Body)
	timestamp := event.Headers[discordbot.TimestampHeader]
	signature := event.Headers[discordbot.SignatureHeader]
	if !discordbot.Authenticate(eventBody, timestamp, signature, h.publicKey) {
		return unauthorizedResponse
	}

	// Parse request from event body
	var req discordbot.Request
	if err := json.Unmarshal(eventBody, &req); err != nil {
		return badRequestResponse
	}

	// Acknowledges a ping
	if req.Type == discordbot.RequestTypePing {
		rsp := discordbot.Response{
			Type: discordbot.ResponseTypePong,
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

	// Start instance if not currently running
	if state, err := h.instanceClient.GetInstanceState(h.instanceId); err != nil {
		h.logger.Print(err)
		return internalErrorResponse

	} else if state == instance.InstancePendingState {
		// TODO: return deferred response and add msg details to SQS
		// OR return error response indicating server is still starting up
		return internalErrorResponse

	} else if state != instance.InstanceRunningState {
		// Attempt to start
		if err := h.instanceClient.StartInstance(h.instanceId); err != nil {
			h.logger.Print(err)
			return internalErrorResponse
		}

		// TODO: return deferred response and add msg details to SQS
		return internalErrorResponse
	}

	// Forward request to server
	return h.forwardToInstance(eventBody, event.Headers)
}

func (h *Handler) loadEnv() error {
	// Get expected env variables
	h.instanceId = os.Getenv(EnvInstanceId)
	publicKey := os.Getenv(EnvPublicKey)
	if h.instanceId == "" || publicKey == "" {
		return MissingEnvErr{envMap: map[string]string{
			EnvInstanceId: h.instanceId,
			EnvPublicKey:  publicKey,
		}}
	}

	// Decode public key
	var err error
	if h.publicKey, err = discordbot.DecodePublicKey(publicKey); err != nil {
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
	endpoint := fmt.Sprintf("http://%s%s", instanceAddress, discordbot.BotEndpoint)

	// Build HTTP request
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		h.logger.Print(err)
		return internalErrorResponse
	}
	req.Header.Set(discordbot.SignatureHeader, headers[discordbot.SignatureHeader])
	req.Header.Set(discordbot.TimestampHeader, headers[discordbot.TimestampHeader])

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
