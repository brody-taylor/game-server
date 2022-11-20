package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"game-server/internal/discordbot/discordlambda"
)

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return discordlambda.Handle(event), nil
}