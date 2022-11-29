package discordbot

import (
	crypto "crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	EnvPublicKey = "DISCORD_PUBLIC_KEY"

	port        = "8080"
	BotEndpoint = "/discord"
)

type BotServer struct {
	publicKey crypto.PublicKey
	mux       http.Handler
}

func New() *BotServer {
	botServer := &BotServer{}

	// Configure server multiplexer
	mux := http.NewServeMux()
	mux.HandleFunc(BotEndpoint, botServer.handler)
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

	// Start listening for requests
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), b.mux)
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (b *BotServer) handler(w http.ResponseWriter, r *http.Request) {
	// Parse request and verify signature
	req, verified, err := parseAndVerifyRequest(r, b.publicKey)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	} else if !verified {
		http.Error(w, "Invalid request signature", http.StatusUnauthorized)
		return
	}

	// Validate
	if req.Type != RequestTypeApplicationCommand ||
		req.Data.Type != RequestDataTypeChatInput {
		http.Error(w, "Unsupported interaction", http.StatusBadRequest)
		return
	}

	// TODO: forward command and get response message
	cmd := req.Data.Name
	rsp := Response{
		Type: ResponseTypeChannelMessage,
		Data: responseData{
			Content: fmt.Sprintf("Recieved command: [%s]", cmd),
		},
	}

	// Write response
	writeResponse(rsp, w)
}

func parseAndVerifyRequest(r *http.Request, publicKey crypto.PublicKey) (req Request, verified bool, err error) {
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

func writeResponse(rsp Response, w http.ResponseWriter) {
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
