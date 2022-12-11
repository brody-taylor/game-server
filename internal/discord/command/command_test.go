package command

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetGameChoice(t *testing.T) {
	game := "gameName"
	tests := []struct {
		name    string
		cmdData discordgo.ApplicationCommandInteractionData
		expErr  bool
	}{
		{
			name: "Happy path",
			cmdData: discordgo.ApplicationCommandInteractionData{
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name:  GameOption,
						Type:  discordgo.ApplicationCommandOptionString,
						Value: game,
					},
				},
			},
		},
		{
			name: "Sad path - Missing game option",
			cmdData: discordgo.ApplicationCommandInteractionData{
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name:  "Other option",
						Type:  discordgo.ApplicationCommandOptionString,
						Value: game,
					},
				},
			},
			expErr: true,
		},
		{
			name:    "Sad path - Nil options",
			cmdData: discordgo.ApplicationCommandInteractionData{},
			expErr: true,
		},
		{
			name: "Sad path - Non-string option type",
			cmdData: discordgo.ApplicationCommandInteractionData{
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name:  GameOption,
						Type:  discordgo.ApplicationCommandOptionInteger,
						Value: game,
					},
				},
			},
			expErr: true,
		},
		{
			name: "Sad path - Non-string value",
			cmdData: discordgo.ApplicationCommandInteractionData{
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name:  GameOption,
						Type:  discordgo.ApplicationCommandOptionString,
						Value: 10,
					},
				},
			},
			expErr: true,
		},
		{
			name: "Sad path - Nil value",
			cmdData: discordgo.ApplicationCommandInteractionData{
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name:  GameOption,
						Type:  discordgo.ApplicationCommandOptionString,
						Value: nil,
					},
				},
			},
			expErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGame, err := GetGameChoice(tt.cmdData)

			if !tt.expErr {
				require.NoError(t, err)
				assert.Equal(t, game, gotGame)
			} else {
				require.Error(t, err)
			}
		})
	}
}
