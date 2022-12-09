package command

import "github.com/bwmarrin/discordgo"

const (
	StartCommand = "start"
	StopCommand  = "stop"
	GameOption   = "game"
)

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        StartCommand,
		Type:        1,
		Description: "Start a game server",
		Options:     []*discordgo.ApplicationCommandOption{gameOption},
	},
	{
		Name:        StopCommand,
		Type:        1,
		Description: "Stop a game server",
		Options:     []*discordgo.ApplicationCommandOption{gameOption},
	},
}

var gameOption = &discordgo.ApplicationCommandOption{
	Name:        GameOption,
	Type:        3,
	Description: "Specify which game",
	Required:    true,
}
