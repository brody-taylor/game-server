package lambda

import (
	crypto "crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"game-server/pkg/aws/instance"
	"game-server/pkg/aws/sqs"
	"game-server/pkg/discord"
)

func Test_New(t *testing.T) {
	h := New()
	assert.NotNil(t, h)
}

func Test_Handle_Ping(t *testing.T) {
	pubKey, privateKey, err := crypto.GenerateKey(nil)
	require.NoError(t, err)

	pubKeyString := hex.EncodeToString(pubKey)
	goodHex := hex.EncodeToString([]byte("randomstring"))
	badHex := "!@#$%^&*()"
	timestamp := time.Now().String()

	pingReq, err := json.Marshal(discordgo.Interaction{
		Type: discordgo.InteractionPing,
	})
	require.NoError(t, err)
	pingReqString := string(pingReq)

	// Set required env variables
	os.Setenv(EnvInstanceId, "instance-id")
	defer os.Unsetenv(EnvInstanceId)
	os.Setenv(EnvSqsUrl, "sqsurl")
	defer os.Unsetenv(EnvSqsUrl)

	testLogger, err := zap.NewDevelopment()
	require.NoError(t, err)
	h := Handler{logger: testLogger}

	tests := []struct {
		name          string
		eventBody     string
		expBody       discordgo.InteractionResponse
		expStatusCode int
		pubKeyEnv     string
		badSignature  string
	}{
		{
			name:          "Happy path - Ping acknowlegement",
			eventBody:     pingReqString,
			expBody:       discord.PingResponse,
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
				discord.SignatureHeader: signature,
				discord.TimestampHeader: timestamp,
			}
			event := events.APIGatewayV2HTTPRequest{
				Headers: eventHeaders,
				Body:    tt.eventBody,
			}

			rsp := h.Handle(event)

			require.Equal(t, tt.expStatusCode, rsp.StatusCode)
			if tt.expStatusCode == http.StatusOK {
				var gotBody discordgo.InteractionResponse
				require.NoError(t, json.Unmarshal([]byte(rsp.Body), &gotBody))
				assert.Equal(t, tt.expBody, gotBody)
			}
		})
	}
}

func Test_Handle_Aws(t *testing.T) {
	pubKey, privateKey, err := crypto.GenerateKey(nil)
	require.NoError(t, err)

	// Build event
	eventBody, err := json.Marshal(discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
	})
	require.NoError(t, err)
	timestamp := time.Now().String()
	signature := hex.EncodeToString(crypto.Sign(privateKey, append([]byte(timestamp), eventBody...)))
	headers := map[string]string{
		discord.SignatureHeader: signature,
		discord.TimestampHeader: timestamp,
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
	sqsUrl := "sqsurl"
	os.Setenv(EnvSqsUrl, sqsUrl)
	defer os.Unsetenv(EnvSqsUrl)

	mockErr := errors.New("mock error")
	tests := []struct {
		name          string
		expStatusCode int
		expRspType    discordgo.InteractionResponseType
		connectErr    error
		getState      string
		getStateErr   error
		startErr      error
		getAddress    string
		getAddressErr error
		sqsSendErr    error
	}{
		{
			name:          "Happy path - Deferred response after starting server",
			expStatusCode: http.StatusOK,
			expRspType:    discord.DeferredResponse.Type,
			getState:      instance.InstanceStoppedState,
		},
		{
			name:          "Happy path - Channel message if command during startup",
			expStatusCode: http.StatusOK,
			expRspType:    discordgo.InteractionResponseChannelMessageWithSource,
			getState:      instance.InstancePendingState,
		},
		{
			name:          "Sad path - AWS connection error",
			expStatusCode: http.StatusInternalServerError,
			connectErr:    mockErr,
		},
		{
			name:          "Sad path - Get state error",
			expStatusCode: http.StatusInternalServerError,
			getStateErr:   mockErr,
		},
		{
			name:          "Sad path - Start error",
			expStatusCode: http.StatusInternalServerError,
			getState:      instance.InstanceStoppedState,
			startErr:      mockErr,
		},
		{
			name:          "Sad path - Get instance address error",
			expStatusCode: http.StatusInternalServerError,
			getState:      instance.InstanceRunningState,
			getAddressErr: mockErr,
		},
		{
			name:          "Sad path - Send to SQS error",
			expStatusCode: http.StatusInternalServerError,
			getState:      instance.InstanceStoppedState,
			sqsSendErr:    mockErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock instance client
			mockInstanceClient := new(instance.MockClient)
			mockInstanceClient.On(instance.ConnectMethod).Return(tt.connectErr)
			mockInstanceClient.On(instance.GetInstanceStateMethod, instanceId).Return(tt.getState, tt.getStateErr)
			mockInstanceClient.On(instance.StartInstanceMethod, instanceId).Return(tt.startErr)
			mockInstanceClient.On(instance.GetInstanceAddressMethod, instanceId).Return(tt.getAddress, tt.getAddressErr)
			awsSession := &session.Session{}
			mockInstanceClient.On(instance.GetSessionMethod).Return(awsSession)

			// Setup mock SQS client
			mockSqsClient := new(sqs.MockClient)
			mockSqsClient.On(sqs.ConnectWithSessionMethod, awsSession).Return()
			mockSqsClient.On(sqs.SendMethod, sqsUrl, event.Body).Return(tt.sqsSendErr)

			testLogger, err := zap.NewDevelopment()
			require.NoError(t, err)
			h := Handler{
				logger:         testLogger,
				instanceClient: mockInstanceClient,
				sqsClient:      mockSqsClient,
			}

			rsp := h.Handle(event)

			// Check HTTP status code
			require.Equal(t, tt.expStatusCode, rsp.StatusCode)

			// Check interaction response
			if tt.expStatusCode == http.StatusOK {
				var interactionRsp discordgo.InteractionResponse
				require.NoError(t, json.Unmarshal([]byte(rsp.Body), &interactionRsp))
				assert.Equal(t, tt.expRspType, interactionRsp.Type)
				if tt.expRspType == discordgo.InteractionResponseChannelMessageWithSource {
					assert.NotEmpty(t, interactionRsp.Data.Content)
				}
			}
		})
	}
}
