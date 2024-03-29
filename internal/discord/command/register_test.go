package command

import (
	"errors"
	"fmt"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"game-server/internal/config"
	"game-server/internal/testing/mockserver"
	"game-server/pkg/discord"
)

func Test_Connect(t *testing.T) {
	testCfg := mockserver.GetConfig(t)

	tests := []struct {
		name   string
		expErr bool
		noEnv  bool
	}{
		{
			name: "Happy path",
		},
		{
			name:   "Sad path - Missing env variables",
			expErr: true,
			noEnv:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(testCfg)

			// Set required env variables
			if !tt.noEnv {
				t.Setenv(EnvApplicationID, "appid")
				t.Setenv(EnvBotToken, "token")
			}

			err := c.Connect()

			if !tt.expErr {
				require.NoError(t, c.Connect())
				assert.NotNil(t, c.discordSession)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func Test_Register(t *testing.T) {
	testCfg := mockserver.GetConfig(t)

	mockErr := errors.New("mock err")
	gameNames := testCfg.GetGameNames()
	cmdNames := make([]string, len(commands))
	for i, cmd := range commands {
		cmdNames[i] = cmd.Name
	}

	tests := []struct {
		name      string
		expErr    string
		createErr error
	}{
		{
			name: "Happy path",
		},
		{
			name:      "Sad path - Failed to register command",
			expErr:    fmt.Sprintf(registerFailErrorFormat, cmdNames),
			createErr: mockErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				logger: config.NewTestLogger(),
				cfg:    testCfg,
				appId:  "appId",
			}

			// Setup mock discord session
			mockSession := new(discord.MockDiscordSession)
			createCall := mockSession.On(discord.SessionApplicationCommandCreateMethod, c.appId, "", mock.Anything)
			createCall.Return(nil, tt.createErr)
			c.discordSession = mockSession

			// Ensure all games are set as options
			createCall.Run(func(args mock.Arguments) {
				cmd := args.Get(2).(*discordgo.ApplicationCommand)
				foundGameOption := false
				missing := make([]string, 0, len(gameNames))
				for _, op := range cmd.Options {
					if op.Name == GameOption {
						foundGameOption = true
						for _, gameName := range gameNames {
							found := false
							for _, choice := range op.Choices {
								if choice.Name == gameName {
									found = true
								}
								break
							}
							if !found {
								missing = append(missing, gameName)
							}
						}
					}
				}
				assert.True(t, foundGameOption, "Command missing game option")
				assert.Emptyf(t, missing, "Command missing game choices: %s", missing)
			})

			err := c.Register()

			if tt.expErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.EqualError(t, err, tt.expErr)
			}
			mockSession.AssertNumberOfCalls(t, discord.SessionApplicationCommandCreateMethod, len(commands))
		})
	}
}

func Test_Clear(t *testing.T) {
	mockErr := errors.New("mock err")
	cmdNames := make([]string, len(commands))
	for i, cmd := range commands {
		cmdNames[i] = cmd.Name
	}

	tests := []struct {
		name       string
		expErr     string
		getCmdsErr error
		deleteErr  error
	}{
		{
			name: "Happy path",
		},
		{
			name:       "Sad path - Failed to get list of registered commands",
			expErr:     mockErr.Error(),
			getCmdsErr: mockErr,
		},
		{
			name:      "Sad path - Failed to delete command",
			expErr:    fmt.Sprintf(removeFailErrorFormat, cmdNames),
			deleteErr: mockErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				logger: config.NewTestLogger(),
				appId:  "appId",
			}

			// Setup mock discord session
			mockSession := new(discord.MockDiscordSession)
			mockSession.On(discord.SessionApplicationCommandsMethod, c.appId, "").Return(commands, tt.getCmdsErr)
			mockSession.On(discord.SessionApplicationCommandDeleteMethod, c.appId, "", mock.Anything).Return(tt.deleteErr)
			c.discordSession = mockSession

			err := c.Clear()

			if tt.expErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.EqualError(t, err, tt.expErr)
			}
			if tt.getCmdsErr == nil {
				mockSession.AssertNumberOfCalls(t, discord.SessionApplicationCommandDeleteMethod, len(commands))
			}
		})
	}
}
