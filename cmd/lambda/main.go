package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	discordlambda "game-server/internal/discord/lambda"
)

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	h := discordlambda.New()
	return h.Handle(event), nil
}
