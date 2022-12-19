package command

import (
	"fmt"
	"os"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"game-server/internal/config"
	"game-server/pkg/discord"
	customError "game-server/pkg/errors"
)

const (
	EnvApplicationID = "APPLICATION_ID"
	EnvBotToken      = "BOT_TOKEN"

	loggerName = "cmd-register"

	registerFailErrorFormat = "following commands failed to register: %s"
	removeFailErrorFormat   = "following commands failed to remove: %s"
)

type Client struct {
	logger *zap.Logger
	cfg    *config.Config

	// Env variables
	appId string
	token string

	discordSession discord.SessionIFace
}

func New(cfg *config.Config) *Client {
	return &Client{
		logger: cfg.Logger.Named(loggerName),
		cfg:    cfg,
	}
}

func (c *Client) Connect() error {
	// Get expected env variables
	if err := c.loadEnv(); err != nil {
		return err
	}

	discordSession, err := discordgo.New(fmt.Sprintf(discord.BotTokenFormat, c.token))
	c.discordSession = discordSession
	return err
}

func (c *Client) Register() error {
	// Build choices for available games
	gameNames := c.cfg.GetGameNames()
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(gameNames))
	for i, name := range gameNames {
		choices[i] = &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: name,
		}
	}

	// Add game choices to each command
	for i := 0; i < len(commands); i++ {
		for j := 0; j < len(commands[i].Options); j++ {
			if commands[i].Options[j].Name == GameOption {
				commands[i].Options[j].Choices = choices
			}
		}
	}

	// Register each command
	c.logger.Info("registering commands", zap.Int("TotalCommands", len(commands)))
	fails := make([]string, 0, len(commands))
	for _, cmd := range commands {
		_, err := c.discordSession.ApplicationCommandCreate(c.appId, "", cmd)
		if err != nil {
			c.logger.Error("could not register command", zap.Error(err), zap.String("cmd", cmd.Name))
			fails = append(fails, cmd.Name)
		}
	}
	if len(fails) > 0 {
		return fmt.Errorf(registerFailErrorFormat, fails)
	}

	c.logger.Info("all commands were registered successfully")
	return nil
}

func (c *Client) Clear() error {
	// Get all currently registerd commands
	cmds, err := c.discordSession.ApplicationCommands(c.appId, "")
	if err != nil {
		return err
	}

	// Delete each command
	c.logger.Info("removing commands", zap.Int("TotalCommands", len(cmds)))
	fails := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		err := c.discordSession.ApplicationCommandDelete(c.appId, "", cmd.ID)
		if err != nil {
			c.logger.Error("could not delete command", zap.Error(err), zap.String("cmd", cmd.Name))
			fails = append(fails, cmd.Name)
		}
	}
	if len(fails) > 0 {
		return fmt.Errorf(removeFailErrorFormat, fails)
	}

	c.logger.Info("all commands were removed successfully")
	return nil
}

func (c *Client) loadEnv() error {
	c.appId = os.Getenv(EnvApplicationID)
	c.token = os.Getenv(EnvBotToken)
	if c.appId == "" || c.token == "" {
		return customError.MissingEnvErr{EnvMap: map[string]string{
			EnvApplicationID: c.appId,
			EnvBotToken:      c.token,
		}}
	}
	return nil
}
