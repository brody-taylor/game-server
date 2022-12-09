package discordbot

import (
	crypto "crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awssqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"game-server/pkg/aws/sqs"
)

func Test_New(t *testing.T) {
	s := New()

	require.NotNil(t, s)
	assert.NotNil(t, s.sqsClient)
	assert.NotNil(t, s.mux)
}

func Test_BotServer_Connect(t *testing.T) {
	pubKey, _, err := crypto.GenerateKey(nil)
	require.NoError(t, err)
	pubKeyString := hex.EncodeToString(pubKey)
	botToken := "bottoken"
	sqsUrl := "sqsurl"

	mockErr := errors.New("mock error")
	tests := []struct {
		name       string
		expErr     string
		pubKey     string
		noEnv      bool
		connectErr error
	}{
		{
			name:   "Happy path",
			pubKey: pubKeyString,
		},
		{
			name:   "Sad path - Missing env variables",
			expErr: "insufficient env variables",
			noEnv:  true,
		},
		{
			name:   "Sad path - Bad public key",
			pubKey: "!@#$%^&*()",
			expErr: "invalid public key",
		},
		{
			name:       "Sad path - AWS connect error",
			pubKey:     pubKeyString,
			expErr:     mockErr.Error(),
			connectErr: mockErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set required env variables
			if !tt.noEnv {
				os.Setenv(EnvPublicKey, tt.pubKey)
				defer os.Unsetenv(EnvPublicKey)
				os.Setenv(EnvBotToken, botToken)
				defer os.Unsetenv(EnvBotToken)
				os.Setenv(EnvSqsUrl, sqsUrl)
				defer os.Unsetenv(EnvSqsUrl)
			}

			// Setup mock SQS client
			mockSqsClient := new(sqs.MockClient)
			mockSqsClient.On(sqs.ConnectMethod).Return(tt.connectErr)

			b := BotServer{
				sqsClient: mockSqsClient,
			}

			err := b.Connect()

			if tt.expErr == "" {
				require.NoError(t, err)
				assert.NotNil(t, b.discordSession)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expErr)
			}
		})
	}
}

func Test_BotServer_CheckMessageQueue(t *testing.T) {
	// Build good mock interaction
	goodReq := &discordgo.Interaction{
		ChannelID: "channelId",
		Type:      discordgo.InteractionApplicationCommand,
		Data: &discordgo.ApplicationCommandInteractionData{
			Name: "command",
		},
	}
	goodReqJson, err := json.Marshal(goodReq)
	require.NoError(t, err)

	// Build mock interaction with an invalid type
	invalidTypeReq := &discordgo.Interaction{
		Type: discordgo.InteractionPing,
	}
	invalidTypeReqJson, err := json.Marshal(invalidTypeReq)
	require.NoError(t, err)

	mockErr := errors.New("mock error")
	tests := []struct {
		name       string
		expErr     string
		recieveErr error
		rspEditErr error
		queuedMsg  *awssqs.Message
	}{
		{
			name:       "Sad path - SQS error recieving",
			expErr:     mockErr.Error(),
			recieveErr: mockErr,
		},
		{
			name:      "Sad path - No message in queue",
			expErr:    "no interaction in deferred message queue",
			queuedMsg: nil,
		},
		{
			name:   "Sad path - Invalid message body",
			expErr: "invalid character",
			queuedMsg: &awssqs.Message{
				Body: aws.String("invalid message"),
			},
		},
		{
			name:   "Sad path - Invalid interaction type",
			expErr: "unsupported interaction type",
			queuedMsg: &awssqs.Message{
				Body: aws.String(string(invalidTypeReqJson)),
			},
		},
		{
			name:   "Sad path - Response edit error",
			expErr: mockErr.Error(),
			queuedMsg: &awssqs.Message{
				Body: aws.String(string(goodReqJson)),
			},
			rspEditErr: mockErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := BotServer{
				sqsUrl: "sqsUrl",
			}

			// Setup mock SQS client
			mockSqsClient := new(sqs.MockClient)
			mockSqsClient.On(sqs.ReceiveMethod, b.sqsUrl).Return(tt.queuedMsg, tt.recieveErr)
			b.sqsClient = mockSqsClient

			// Setup mock discord session
			mockSession := new(MockDiscordSession)
			mockSession.On(SessionInteractionResponseEditMethod, mock.Anything, mock.Anything).Return(nil, tt.rspEditErr)
			b.discordSession = mockSession

			err := b.Run()

			if tt.expErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expErr)
			}
		})
	}
}
