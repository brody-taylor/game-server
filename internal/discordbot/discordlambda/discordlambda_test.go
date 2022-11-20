package discordlambda_test

import (
	crypto "crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"game-server/internal/discordbot"
	"game-server/internal/discordbot/discordlambda"
)

func Test_Handle(t *testing.T) {
	pubKey, privateKey, err := crypto.GenerateKey(nil)
	require.NoError(t, err)

	pubKeyString := hex.EncodeToString(pubKey)
	goodHex := hex.EncodeToString([]byte("randomstring"))
	badHex := "!@#$%^&*()"

	pingReq, err := json.Marshal(discordbot.Request{Type: discordbot.RequestTypePing})
	require.NoError(t, err)
	pingReqString := string(pingReq)

	tests := []struct {
		name          string
		eventBody     string
		expBody       discordbot.Response
		expStatusCode int
		pubKeyEnv     string
		badSignature  string
	}{
		{
			name:          "Happy path - Ping acknowlegement",
			eventBody:     pingReqString,
			expBody:       discordbot.Response{Type: discordbot.ResponseTypePong},
			expStatusCode: http.StatusOK,
			pubKeyEnv:     pubKeyString,
		},
		{
			name:          "Sad path - Missing public key",
			eventBody:     pingReqString,
			expStatusCode: http.StatusUnauthorized,
			pubKeyEnv:     "",
		},
		{
			name:          "Sad path - Non-hexidecimal public key",
			eventBody:     pingReqString,
			expStatusCode: http.StatusUnauthorized,
			pubKeyEnv:     badHex,
		},
		{
			name:          "Sad path - Non-hexidecimal signature",
			eventBody:     pingReqString,
			expStatusCode: http.StatusUnauthorized,
			pubKeyEnv:     pubKeyString,
			badSignature:  badHex,
		},
		{
			name:          "Sad path - Bad signature",
			eventBody:     pingReqString,
			expStatusCode: http.StatusUnauthorized,
			pubKeyEnv:     pubKeyString,
			badSignature:  goodHex,
		},
		{
			name:          "Sad path - Bad event body",
			eventBody:     "",
			expStatusCode: http.StatusBadRequest,
			pubKeyEnv:     pubKeyString,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate request signature
			timestamp := time.Now().String()
			signature := hex.EncodeToString(crypto.Sign(privateKey, []byte(timestamp+tt.eventBody)))
			if tt.badSignature != "" {
				signature = tt.badSignature
			}

			// Set public key env
			if tt.pubKeyEnv != "" {
				os.Setenv(discordlambda.PublicKeyEnv, tt.pubKeyEnv)
				defer os.Unsetenv(discordlambda.PublicKeyEnv)
			}

			// Setup event
			eventHeaders := map[string]string{
				discordlambda.SignatureHeader: signature,
				discordlambda.TimestampHeader: timestamp,
			}
			event := events.APIGatewayV2HTTPRequest{
				Headers: eventHeaders,
				Body:    tt.eventBody,
			}

			rsp := discordlambda.Handle(event)

			require.Equal(t, tt.expStatusCode, rsp.StatusCode)
			if tt.expStatusCode == http.StatusOK {
				var gotBody discordbot.Response
				require.NoError(t, json.Unmarshal([]byte(rsp.Body), &gotBody))
				assert.Equal(t, tt.expBody, gotBody)
			}
		})
	}
}
