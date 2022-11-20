package discordlambda

import (
	crypto "crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"

	"game-server/internal/discordbot"
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
)

func Handle(event events.APIGatewayV2HTTPRequest) events.APIGatewayV2HTTPResponse {
	// Verify request signature
	if !verify(event) {
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

	// TODO: start server if not running and await startup (might need to respond with a deferred message)

	// TODO: forward request to server
	return events.APIGatewayV2HTTPResponse{}
}

const (
	PublicKeyEnv = "PUBLIC_KEY"

	SignatureHeader = "x-signature-ed25519"
	TimestampHeader = "x-signature-timestamp"
)

func verify(event events.APIGatewayV2HTTPRequest) bool {
	// Get public key
	var publicKey crypto.PublicKey
	if key, ok := os.LookupEnv(PublicKeyEnv); !ok {
		return false
	} else {
		var err error
		if publicKey, err = hex.DecodeString(key); err != nil {
			return false
		}
	}

	signature, err := hex.DecodeString(event.Headers[SignatureHeader])
	if err != nil {
		return false
	}
	timestamp := event.Headers[TimestampHeader]
	msg := []byte(timestamp + event.Body)

	return crypto.Verify(publicKey, msg, signature)
}
