package discordbot

import (
	crypto "crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/bwmarrin/discordgo"

	"game-server/pkg/aws/sqs"
	customError "game-server/pkg/errors"
)

const (
	EnvPublicKey = "DISCORD_PUBLIC_KEY"
	EnvBotToken  = "DISCORD_BOT_TOKEN"
	EnvSqsUrl    = "MESSAGE_QUEUE_URL"

	BotTokenFormat = "Bot %s"

	port        = "8080"
	BotEndpoint = "/discord"
)

type BotServer struct {
	mux http.Handler

	// Env variables
	publicKey crypto.PublicKey
	token     string
	sqsUrl    string

	discordSession SessionIFace
	channelId      string

	// AWS
	sqsClient sqs.ClientIFace
}

func New() *BotServer {
	botServer := &BotServer{
		sqsClient: sqs.New(),
	}

	// Configure server multiplexer
	mux := http.NewServeMux()
	mux.HandleFunc(BotEndpoint, botServer.eventHandler)
	botServer.mux = mux

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
	discordSession, err := discordgo.New(fmt.Sprintf(BotTokenFormat, b.token))
	b.discordSession = discordSession
	return err
}

func (b *BotServer) Run() error {
	// Handle any queued messages
	if err := b.checkMessageQueue(); err != nil {
		return err
	}

	// Start listening for requests
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), b.mux)
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
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
	if b.publicKey, err = DecodePublicKey(publicKey); err != nil {
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
	interactionRsp, err := b.reqHandler(req)
	if err != nil {
		return err
	}

	// Update deferred response
	updatedRsp := &discordgo.WebhookEdit{
		Content:         &interactionRsp.Data.Content,
		Components:      &interactionRsp.Data.Components,
		Embeds:          &interactionRsp.Data.Embeds,
		Files:           interactionRsp.Data.Files,
		AllowedMentions: interactionRsp.Data.AllowedMentions,
	}
	_, err = b.discordSession.InteractionResponseEdit(req, updatedRsp)
	return err
}

func (b *BotServer) eventHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request and verify signature
	req, verified, err := parseAndVerifyRequest(r, b.publicKey)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	} else if !verified {
		http.Error(w, "Invalid request signature", http.StatusUnauthorized)
		return
	}

	// Forward to request handler
	rsp, err := b.reqHandler(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	// Write response
	writeResponse(rsp, w)
}

func (b *BotServer) reqHandler(req *discordgo.Interaction) (*discordgo.InteractionResponse, error) {
	// Validate interaction
	if req.Type != discordgo.InteractionApplicationCommand {
		return nil, errors.New("unsupported interaction type")
	}

	// TODO: forward command to game server client
	reqData := req.ApplicationCommandData()
	cmd := reqData.Name
	rsp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Recieved command: [%s]", cmd),
		},
	}

	return rsp, nil
}

func parseAndVerifyRequest(r *http.Request, publicKey crypto.PublicKey) (req *discordgo.Interaction, verified bool, err error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return req, verified, err
	}

	timestamp := r.Header.Get(TimestampHeader)
	signature := r.Header.Get(SignatureHeader)
	if verified = Authenticate(body, timestamp, signature, publicKey); !verified {
		return req, verified, nil
	}

	err = json.Unmarshal(body, &req)
	return req, verified, err
}

func writeResponse(rsp *discordgo.InteractionResponse, w http.ResponseWriter) {
	jsonRsp, err := json.Marshal(rsp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(jsonRsp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
