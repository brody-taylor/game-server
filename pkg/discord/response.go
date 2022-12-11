package discord

import (
	"encoding/json"

	"github.com/bwmarrin/discordgo"
)

var (
	PingResponse = discordgo.InteractionResponse{
		Type: discordgo.InteractionResponsePong,
	}
	DeferredResponse = discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}

	PingResponseJson     []byte
	DeferredResponseJson []byte
)

// Marshal JSON for common responses
func init() {
	var err error

	PingResponseJson, err = json.Marshal(PingResponse)
	if err != nil {
		panic(err)
	}

	DeferredResponseJson, err = json.Marshal(DeferredResponse)
	if err != nil {
		panic(err)
	}
}
