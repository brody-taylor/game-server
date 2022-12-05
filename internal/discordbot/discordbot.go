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
)

const (
	EnvPublicKey = "DISCORD_PUBLIC_KEY"
	EnvSqsUrl    = "MESSAGE_QUEUE_URL"

	port        = "8080"
	BotEndpoint = "/discord"
)

type BotServer struct {
	publicKey crypto.PublicKey
	mux       http.Handler

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

func (b *BotServer) Run() error {
	// Get public key
	if publicKey, err := DecodePublicKey(os.Getenv(EnvPublicKey)); err != nil {
		return err
	} else {
		b.publicKey = publicKey
	}

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

func (b *BotServer) checkMessageQueue() error {
	// Connect AWS session
	if err := b.sqsClient.Connect(); err != nil {
		return err
	}

	// Check for any queued messages
	msg, err := b.sqsClient.Receive(os.Getenv(EnvSqsUrl))
	if err != nil || msg == nil {
		return err
	}

	// Parse request from message
	var req discordgo.Interaction
	if err := json.Unmarshal([]byte(*msg.Body), &req); err != nil {
		return err
	}

	// Forward to request handler
	_, err = b.reqHandler(req)
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

func (b *BotServer) reqHandler(req discordgo.Interaction) (discordgo.InteractionResponse, error) {
	var rsp discordgo.InteractionResponse

	// Validate
	if req.Type != discordgo.InteractionApplicationCommand {
		return rsp, errors.New("unsupported interaction type")
	}
	
	// TODO: forward command and get response message
	reqData := req.ApplicationCommandData()
	cmd := reqData.Name
	rsp = discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Recieved command: [%s]", cmd),
		},
	}

	return rsp, nil
}

func parseAndVerifyRequest(r *http.Request, publicKey crypto.PublicKey) (req discordgo.Interaction, verified bool, err error) {
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

func writeResponse(rsp discordgo.InteractionResponse, w http.ResponseWriter) {
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
