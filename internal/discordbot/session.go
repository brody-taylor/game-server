package discordbot

import "github.com/bwmarrin/discordgo"

// Ensure SessionIFace is implemented by discordgo.Session
var _ SessionIFace = (*discordgo.Session)(nil)

type SessionIFace interface {
	// Command registration
	ApplicationCommandCreate(appID string, guildID string, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error)
	ApplicationCommands(appID string, guildID string) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommandDelete(appID string, guildID string, cmdID string) error

	// Interactions
	InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error
	InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error)
}
