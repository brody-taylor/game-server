package bot

import (
	"context"
	crypto "crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"game-server/internal/config"
	"game-server/internal/discord/command"
	"game-server/internal/gameserver"
	"game-server/pkg/aws/sqs"
	"game-server/pkg/discord"
	customError "game-server/pkg/errors"
)

const (
	EnvPublicKey = "DISCORD_PUBLIC_KEY"
	EnvBotToken  = "DISCORD_BOT_TOKEN"
	EnvSqsUrl    = "MESSAGE_QUEUE_URL"

	loggerName = "discord-bot"

	port        = "8080"
	BotEndpoint = "/discord"
)

type BotServer struct {
	srv    *http.Server
	logger *zap.Logger

	// Env variables
	publicKey crypto.PublicKey
	token     string
	sqsUrl    string

	gameClient gameserver.ClientIFace

	discordSession discord.SessionIFace
	channelId      string

	// AWS
	sqsClient sqs.ClientIFace
}

func New(cfg *config.Config, gameClient gameserver.ClientIFace) *BotServer {
	botServer := &BotServer{
		logger:     cfg.Logger.Named(loggerName),
		gameClient: gameClient,
		sqsClient:  sqs.New(),
	}

	// Configure server multiplexer
	mux := http.NewServeMux()
	mux.HandleFunc(BotEndpoint, botServer.eventHandler)

	botServer.srv = &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}

	return botServer
}

func (b *BotServer) Connect() error {
	// Get expected env variables
	if err := b.loadEnv(); err != nil {
		return err
	}

	// Connect AWS session
	if err := b.sqsClient.Connect(); err != nil {
		return err
	}

	// Connect to discord session
	discordSession, err := discordgo.New(fmt.Sprintf(discord.BotTokenFormat, b.token))
	b.discordSession = discordSession
	return err
}

func (b *BotServer) Run() error {
	// Handle any queued messages
	b.logger.Info("checking deferred message queue")
	if err := b.checkMessageQueue(); err != nil {
		return err
	}

	// Start listening for requests
	b.logger.Info("now listening", zap.String("port", port))
	err := b.srv.ListenAndServe()
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (b *BotServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()
	return b.srv.Shutdown(ctx)
}

func (b *BotServer) loadEnv() error {
	// Get expected env variables
	publicKey := os.Getenv(EnvPublicKey)
	b.token = os.Getenv(EnvBotToken)
	b.sqsUrl = os.Getenv(EnvSqsUrl)
	if publicKey == "" || b.token == "" || b.sqsUrl == "" {
		return customError.MissingEnvErr{EnvMap: map[string]string{
			EnvPublicKey: publicKey,
			EnvBotToken:  b.token,
			EnvSqsUrl:    b.sqsUrl,
		}}
	}

	// Decode public key
	var err error
	if b.publicKey, err = discord.DecodePublicKey(publicKey); err != nil {
		return fmt.Errorf("invalid public key")
	}

	return nil
}

func (b *BotServer) checkMessageQueue() error {
	// Check for any queued messages
	msg, err := b.sqsClient.Receive(b.sqsUrl)
	if err != nil {
		return err
	} else if msg == nil {
		return errors.New("no interaction in deferred message queue")
	}

	// Parse request from message
	var req *discordgo.Interaction
	if err := json.Unmarshal([]byte(*msg.Body), &req); err != nil {
		return err
	}

	// Set channel ID from the interaction that launched the service
	b.channelId = req.ChannelID

	// Forward to request handler
	interactionResp, err := b.reqHandler(req)
	if err != nil {
		return err
	}

	// Update deferred response
	updatedResp := &discordgo.WebhookEdit{
		Content:         &interactionResp.Data.Content,
		Components:      &interactionResp.Data.Components,
		Embeds:          &interactionResp.Data.Embeds,
		Files:           interactionResp.Data.Files,
		AllowedMentions: interactionResp.Data.AllowedMentions,
	}
	_, err = b.discordSession.InteractionResponseEdit(req, updatedResp)
	return err
}

func (b *BotServer) eventHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request and verify signature
	req, verified, err := parseAndVerifyRequest(r, b.publicKey)
	if err != nil {
		b.logger.Error("recieved bad request", zap.Error(err))
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	} else if !verified {
		b.logger.Error("recieved unauthorized request", zap.Error(err))
		http.Error(w, "Invalid request signature", http.StatusUnauthorized)
		return
	}
	b.logger.Info("recieved request")

	// Forward to request handler
	resp, err := b.reqHandler(req)
	if err != nil {
		b.logger.Error("failed to handle request", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Write response
	writeResponse(resp, w)
}

func (b *BotServer) reqHandler(req *discordgo.Interaction) (*discordgo.InteractionResponse, error) {
	// Validate interaction and get command data
	if req.Type != discordgo.InteractionApplicationCommand {
		return nil, errors.New("unsupported interaction type")
	}
	reqData := req.ApplicationCommandData()
	reqGame, err := command.GetGameChoice(reqData)
	if err != nil {
		return nil, err
	}

	// Handle command
	switch reqData.Name {
	case command.StartCommand:
		return b.startHandler(reqGame)
	case command.StopCommand:
		return b.stopHandler(reqGame)
	}
	return nil, fmt.Errorf("unsupported command: [%s]", reqData.Name)
}

func (b *BotServer) startHandler(startGame string) (*discordgo.InteractionResponse, error) {
	// Ensure a game is not already running
	if runningGame, isRunning := b.gameClient.IsRunning(); isRunning {
		b.logger.Info("recieved start request while game is running", zap.String("requestGame", startGame), zap.String("runningGame", runningGame))
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Cannot start %s server because %s is already running", startGame, runningGame),
			},
		}, nil
	}

	// Start server
	go func(game string) {
		if err := b.gameClient.Run(game); err != nil {
			b.logger.Error("failed to start game server", zap.Error(err), zap.String("game", game))
			msg := fmt.Sprintf("Could not start %s server", game)
			b.messageChannel(msg)
		}
	}(startGame)

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Starting %s game server", startGame),
		},
	}, nil
}

func (b *BotServer) stopHandler(stopGame string) (*discordgo.InteractionResponse, error) {
	// Ensure requested game is currently running
	runningGame, isRunning := b.gameClient.IsRunning()
	if !isRunning || runningGame != stopGame {
		b.logger.Info("recieved stop request for game that is not running", zap.String("requestGame", stopGame), zap.String("runningGame", runningGame))
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Cannot stop %s server because it is not currently running", stopGame),
			},
		}, nil
	}

	// Stop server
	go func(game string) {
		if err := b.gameClient.Stop(); err != nil {
			b.logger.Error("failed to stop game server", zap.Error(err), zap.String("game", game))
			msg := fmt.Sprintf("Could not stop %s server", game)
			b.messageChannel(msg)
		}
	}(stopGame)

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("%s server is shutting down", stopGame),
		},
	}, nil
}

func (b *BotServer) messageChannel(msg string) bool {
	if _, err := b.discordSession.ChannelMessageSend(b.channelId, msg); err != nil {
		b.logger.Error("could not send channel message", zap.Error(err), zap.String("channelMsg", msg))
		return false
	}
	return true
}

func parseAndVerifyRequest(r *http.Request, publicKey crypto.PublicKey) (req *discordgo.Interaction, verified bool, err error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return req, verified, err
	}

	timestamp := r.Header.Get(discord.TimestampHeader)
	signature := r.Header.Get(discord.SignatureHeader)
	if verified = discord.Authenticate(body, timestamp, signature, publicKey); !verified {
		return req, verified, nil
	}

	err = json.Unmarshal(body, &req)
	return req, verified, err
}

func writeResponse(resp *discordgo.InteractionResponse, w http.ResponseWriter) {
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(jsonResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
