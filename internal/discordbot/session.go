package discordbot

import "github.com/bwmarrin/discordgo"

// Ensure SessionIFace is implemented by discordgo.Session
var _ SessionIFace = (*discordgo.Session)(nil)

type SessionIFace interface {
	ApplicationCommandCreate(appID string, guildID string, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error)
	ApplicationCommands(appID string, guildID string) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommandDelete(appID string, guildID string, cmdID string) error
}
