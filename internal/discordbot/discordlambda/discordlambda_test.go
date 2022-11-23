package discordlambda

import (
	crypto "crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"game-server/internal/discordbot"
	"game-server/pkg/aws/instance"
)

func Test_Handle_Ping(t *testing.T) {
	pubKey, privateKey, err := crypto.GenerateKey(nil)
	require.NoError(t, err)

	pubKeyString := hex.EncodeToString(pubKey)
	goodHex := hex.EncodeToString([]byte("randomstring"))
	badHex := "!@#$%^&*()"
	timestamp := time.Now().String()

	pingReq, err := json.Marshal(discordbot.Request{Type: discordbot.RequestTypePing})
	require.NoError(t, err)
	pingReqString := string(pingReq)

	// Set required env variables
	os.Setenv(EnvInstanceId, "instance-id")
	defer os.Unsetenv(EnvInstanceId)

	h := Handler{logger: log.Default()}

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
			expStatusCode: http.StatusInternalServerError,
			pubKeyEnv:     "",
		},
		{
			name:          "Sad path - Non-hexidecimal public key",
			eventBody:     pingReqString,
			expStatusCode: http.StatusInternalServerError,
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
			signature := hex.EncodeToString(crypto.Sign(privateKey, []byte(timestamp+tt.eventBody)))
			if tt.badSignature != "" {
				signature = tt.badSignature
			}

			// Set public key env
			if tt.pubKeyEnv != "" {
				os.Setenv(EnvPublicKey, tt.pubKeyEnv)
				defer os.Unsetenv(EnvPublicKey)
			}

			// Setup event
			eventHeaders := map[string]string{
				SignatureHeader: signature,
				TimestampHeader: timestamp,
			}
			event := events.APIGatewayV2HTTPRequest{
				Headers: eventHeaders,
				Body:    tt.eventBody,
			}

			rsp := h.Handle(event)

			require.Equal(t, tt.expStatusCode, rsp.StatusCode)
			if tt.expStatusCode == http.StatusOK {
				var gotBody discordbot.Response
				require.NoError(t, json.Unmarshal([]byte(rsp.Body), &gotBody))
				assert.Equal(t, tt.expBody, gotBody)
			}
		})
	}
}

func Test_Handle_Instance(t *testing.T) {
	// Build event
	pubKey, privateKey, err := crypto.GenerateKey(nil)
	require.NoError(t, err)
	eventBody, err := json.Marshal(discordbot.Request{Type: 2})
	require.NoError(t, err)
	timestamp := time.Now().String()
	signature := hex.EncodeToString(crypto.Sign(privateKey, append([]byte(timestamp), eventBody...)))
	headers := map[string]string{
		SignatureHeader: signature,
		TimestampHeader: timestamp,
	}
	event := events.APIGatewayV2HTTPRequest{
		Headers: headers,
		Body:    string(eventBody),
	}

	// Set required env variables
	instanceId := "instance-id"
	os.Setenv(EnvInstanceId, instanceId)
	defer os.Unsetenv(EnvInstanceId)
	os.Setenv(EnvPublicKey, hex.EncodeToString(pubKey))

	mockErr := errors.New("mock error")
	tests := []struct {
		name          string
		expStatusCode int
		connectErr    error
		getState      string
		getStateErr   error
		startErr      error
	}{
		{
			name:          "Sad Path - AWS Connection Error",
			expStatusCode: http.StatusInternalServerError,
			connectErr:    mockErr,
		},
		{
			name:          "Sad Path - Get State Error",
			expStatusCode: http.StatusInternalServerError,
			getStateErr:   mockErr,
		},
		{
			name:          "Sad Path - Start Error",
			expStatusCode: http.StatusInternalServerError,
			getState:      instance.InstanceStoppedState,
			startErr:      mockErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock instance client
			mockInstanceClient := new(instance.MockClient)
			mockInstanceClient.On(instance.ConnectMethod).Return(tt.connectErr)
			mockInstanceClient.On(instance.GetInstanceStateMethod, instanceId).Return(tt.getState, tt.getStateErr)
			mockInstanceClient.On(instance.StartInstanceMethod, instanceId).Return(tt.startErr)
			h := Handler{
				logger:         log.Default(),
				instanceClient: mockInstanceClient,
			}

			rsp := h.Handle(event)

			require.Equal(t, tt.expStatusCode, rsp.StatusCode)
		})
	}
}
