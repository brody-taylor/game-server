package bot

import (
	crypto "crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awssqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"game-server/internal/config"
	"game-server/internal/discord/command"
	"game-server/internal/gameserver"
	"game-server/pkg/aws/sqs"
	"game-server/pkg/discord"
)

func Test_New(t *testing.T) {
	testCfg, err := config.NewTestConfig()
	require.NoError(t, err)

	s := New(testCfg, new(gameserver.MockClient))

	require.NotNil(t, s)
	assert.NotNil(t, s.sqsClient)
	assert.NotNil(t, s.gameClient)
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
				t.Setenv(EnvPublicKey, tt.pubKey)
				t.Setenv(EnvBotToken, botToken)
				t.Setenv(EnvSqsUrl, sqsUrl)
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
	goodReqGame := "gameName"
	goodReq := &discordgo.Interaction{
		ChannelID: "channelId",
		Type:      discordgo.InteractionApplicationCommand,
		Data: &discordgo.ApplicationCommandInteractionData{
			Name: command.StartCommand,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{
					Name:  command.GameOption,
					Type:  discordgo.ApplicationCommandOptionString,
					Value: goodReqGame,
				},
			},
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

	// Setup mock game server client
	mockGameClient := new(gameserver.MockClient)
	mockGameClient.On(gameserver.IsRunningMethod).Return("", false)
	mockGameClient.On(gameserver.RunMethod, goodReqGame).Return(nil)

	mockErr := errors.New("mock error")
	tests := []struct {
		name        string
		expErr      string
		recieveErr  error
		respEditErr error
		queuedMsg   *awssqs.Message
	}{
		{
			name: "Happy path",
			queuedMsg: &awssqs.Message{
				Body: aws.String(string(goodReqJson)),
			},
		},
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
			name:   "Sad path - Request handler error",
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
			respEditErr: mockErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := BotServer{
				sqsUrl:     "sqsUrl",
				gameClient: mockGameClient,
			}

			// Setup mock SQS client
			mockSqsClient := new(sqs.MockClient)
			mockSqsClient.On(sqs.ReceiveMethod, b.sqsUrl).Return(tt.queuedMsg, tt.recieveErr)
			b.sqsClient = mockSqsClient

			// Setup mock discord session
			mockSession := new(discord.MockDiscordSession)
			mockSession.On(discord.SessionInteractionResponseEditMethod, mock.Anything, mock.Anything).Return(nil, tt.respEditErr)
			b.discordSession = mockSession

			err := b.checkMessageQueue()

			if tt.expErr == "" {
				require.NoError(t, err)
				mockSqsClient.AssertCalled(t, sqs.ReceiveMethod, b.sqsUrl)
				mockSession.AssertCalled(t, discord.SessionInteractionResponseEditMethod, mock.Anything, mock.Anything)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expErr)
			}
		})
	}
}

func Test_BotServer_RequestHandler(t *testing.T) {
	testCfg, err := config.NewTestConfig()
	require.NoError(t, err)

	gameName := "gameName"
	chanId := "channelId"
	mockErr := errors.New("mock error")
	tests := []struct {
		name        string
		req         *discordgo.Interaction
		expContent  string
		expErr      string
		expChanMsg  string
		isRunning   bool
		runningGame string
		runErr      error
		stopErr     error
	}{
		{
			name: "Happy path - Start command",
			req: &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: command.StartCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  command.GameOption,
							Type:  discordgo.ApplicationCommandOptionString,
							Value: gameName,
						},
					},
				},
			},
			expContent: fmt.Sprintf("Starting %s game server", gameName),
			isRunning:  false,
		},
		{
			name: "Happy path - Start command with game currently running",
			req: &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: command.StartCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  command.GameOption,
							Type:  discordgo.ApplicationCommandOptionString,
							Value: gameName,
						},
					},
				},
			},
			expContent:  "is already running",
			isRunning:   true,
			runningGame: "otherGame",
		},
		{
			name: "Happy path - Stop command",
			req: &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: command.StopCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  command.GameOption,
							Type:  discordgo.ApplicationCommandOptionString,
							Value: gameName,
						},
					},
				},
			},
			expContent:  "server is shutting down",
			isRunning:   true,
			runningGame: gameName,
		},
		{
			name: "Happy path - Stop command with different game running",
			req: &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: command.StopCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  command.GameOption,
							Type:  discordgo.ApplicationCommandOptionString,
							Value: gameName,
						},
					},
				},
			},
			expContent:  "it is not currently running",
			isRunning:   true,
			runningGame: "otherGame",
		},
		{
			name: "Happy path - Stop command with no game running",
			req: &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: command.StopCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  command.GameOption,
							Type:  discordgo.ApplicationCommandOptionString,
							Value: gameName,
						},
					},
				},
			},
			expContent: "it is not currently running",
			isRunning:  false,
		},
		{
			name: "Sad path - Bad interaction type",
			req: &discordgo.Interaction{
				Type: discordgo.InteractionPing,
			},
			expErr: "unsupported interaction type",
		},
		{
			name: "Sad path - Missing game choice",
			req: &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: command.StartCommand,
				},
			},
			expErr: "command missing game choice",
		},
		{
			name: "Sad path - Game server run error",
			req: &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: command.StartCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  command.GameOption,
							Type:  discordgo.ApplicationCommandOptionString,
							Value: gameName,
						},
					},
				},
			},
			expChanMsg: fmt.Sprintf("Could not start %s server", gameName),
			runErr:     mockErr,
		},
		{
			name: "Sad path - Game server stop error",
			req: &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: command.StopCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  command.GameOption,
							Type:  discordgo.ApplicationCommandOptionString,
							Value: gameName,
						},
					},
				},
			},
			expChanMsg:  fmt.Sprintf("Could not stop %s server", gameName),
			isRunning:   true,
			runningGame: gameName,
			stopErr:     mockErr,
		},
		{
			name: "Sad path - Unknown command",
			req: &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: "badCommand",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  command.GameOption,
							Type:  discordgo.ApplicationCommandOptionString,
							Value: gameName,
						},
					},
				},
			},
			expErr: "unsupported command",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock game server client
			mockGameClient := new(gameserver.MockClient)
			mockGameClient.On(gameserver.IsRunningMethod).Return(tt.runningGame, tt.isRunning)
			mockGameClient.On(gameserver.RunMethod, gameName).Return(tt.runErr)
			mockGameClient.On(gameserver.StopMethod).Return(tt.stopErr)

			// Setup mock discord session
			msgCall := make(chan string)
			mockSession := new(discord.MockDiscordSession)
			chanMsgCall := mockSession.On(discord.SessionChannelMessageSendMethod, chanId, tt.expChanMsg)
			chanMsgCall.Run(func(args mock.Arguments) {
				msgCall <- args.String(1)
			})
			chanMsgCall.Return(nil, nil)

			b := &BotServer{
				logger:         testCfg.Logger,
				channelId:      chanId,
				gameClient:     mockGameClient,
				discordSession: mockSession,
			}

			resp, err := b.reqHandler(tt.req)

			if tt.expErr == "" {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Contains(t, resp.Data.Content, tt.expContent)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expErr)
			}

			// Check for expected channel message
			if tt.expChanMsg != "" {
				select {
				case msg := <-msgCall:
					assert.Equal(t, tt.expChanMsg, msg)
				case <-time.After(time.Second):
					assert.Fail(t, "Expected channel message was not recieved")
				}
			}
		})
	}
}
