package discordbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	port = "8080"
	botEndpoint = "/discord"
)

type BotServer struct {
	mux http.Handler
} 

func New() *BotServer {
	// Configure server multiplexer
	mux := http.NewServeMux()
	mux.HandleFunc(botEndpoint, handler)

	return &BotServer{
		mux: mux,
	}
}

func (b *BotServer) Run() error {
	// Start listening for requests
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), b.mux)
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func handler(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req, err := parseRequest(r)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
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

func parseRequest(r *http.Request) (Request, error) {
	var req Request

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return req, err
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		return req, err
	}

	return req, nil
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
