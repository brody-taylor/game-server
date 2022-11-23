package discordlambda

import (
	crypto "crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"game-server/internal/discordbot"
	"game-server/pkg/aws/instance"
)

const (
	EnvInstanceId = "INSTANCE_ID"
	EnvPublicKey  = "PUBLIC_KEY"

	SignatureHeader = "x-signature-ed25519"
	TimestampHeader = "x-signature-timestamp"
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
	// Env variables
	publicKey  crypto.PublicKey
	instanceId string

	// AWS
	instanceClient instance.ClientIFace
}

func New() *Handler {
	return &Handler{
		instanceClient: instance.New(),
	}
}

func (h *Handler) Handle(event events.APIGatewayV2HTTPRequest) events.APIGatewayV2HTTPResponse {
	if err := h.loadEnv(); err != nil {
		fmt.Fprint(os.Stderr, err)
		return internalErrorResponse
	}

	// Verify request signature
	if !h.verify(event) {
		return unauthorizedResponse
	}

	// Parse request from event body
	eventBody := []byte(event.Body)
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
		fmt.Fprint(os.Stderr, err)
		return internalErrorResponse
	}

	// Start instance if not currently running
	if state, err := h.instanceClient.GetInstanceState(h.instanceId); err != nil {
		fmt.Fprint(os.Stderr, err)
		return internalErrorResponse

	} else if state == instance.InstancePendingState {
		// TODO: return deferred response and add msg details to SQS
		// OR return error response indicating server is still starting up
		return internalErrorResponse

	} else if state != instance.InstanceRunningState {
		// Attempt to start
		if err := h.instanceClient.StartInstance(h.instanceId); err != nil {
			fmt.Fprint(os.Stderr, err)
			return internalErrorResponse
		}

		// TODO: return deferred response and add msg details to SQS
		return internalErrorResponse
	}

	// TODO: forward request to server
	return internalErrorResponse
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
	h.publicKey, err = hex.DecodeString(publicKey)
	if err != nil {
		return fmt.Errorf("invalid public key")
	}

	return nil
}

func (h *Handler) verify(event events.APIGatewayV2HTTPRequest) bool {
	signature, err := hex.DecodeString(event.Headers[SignatureHeader])
	if err != nil {
		return false
	}
	timestamp := event.Headers[TimestampHeader]
	msg := []byte(timestamp + event.Body)

	return crypto.Verify(h.publicKey, msg, signature)
}
