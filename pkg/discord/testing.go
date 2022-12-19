package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/mock"
)

const (
	SessionApplicationCommandCreateMethod = "ApplicationCommandCreate"
	SessionApplicationCommandsMethod      = "ApplicationCommands"
	SessionApplicationCommandDeleteMethod = "ApplicationCommandDelete"
	SessionChannelMessageSendMethod       = "ChannelMessageSend"
	SessionInteractionRespondMethod       = "InteractionRespond"
	SessionInteractionResponseEditMethod  = "InteractionResponseEdit"
)

// Ensure MockDiscordSession implements SessionIFace
var _ SessionIFace = (*MockDiscordSession)(nil)

type MockDiscordSession struct {
	mock.Mock
}

func (m *MockDiscordSession) ApplicationCommandCreate(appID string, guildID string, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
	args := m.Called(appID, guildID, cmd)
	if respCmd := args.Get(0); respCmd != nil {
		return respCmd.(*discordgo.ApplicationCommand), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockDiscordSession) ApplicationCommands(appID string, guildID string) ([]*discordgo.ApplicationCommand, error) {
	args := m.Called(appID, guildID)
	return args.Get(0).([]*discordgo.ApplicationCommand), args.Error(1)
}

func (m *MockDiscordSession) ApplicationCommandDelete(appID string, guildID string, cmdID string) error {
	args := m.Called(appID, guildID, cmdID)
	return args.Error(0)
}

func (m *MockDiscordSession) ChannelMessageSend(channelID string, content string) (*discordgo.Message, error) {
	args := m.Called(channelID, content)
	if respMsg := args.Get(0); respMsg != nil {
		return respMsg.(*discordgo.Message), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockDiscordSession) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	args := m.Called(interaction, resp)
	return args.Error(0)
}

func (m *MockDiscordSession) InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
	args := m.Called(interaction, newresp)
	if respMsg := args.Get(0); respMsg != nil {
		return respMsg.(*discordgo.Message), args.Error(1)
	}
	return nil, args.Error(1)
}
