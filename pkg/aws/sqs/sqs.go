package sqs

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

const (
	EnvRegion = "AWS_REGION"

	groupId   = "default"
)

// Ensure Client implements ClientIFace
var _ ClientIFace = (*Client)(nil)

type ClientIFace interface {
	Connect() error
	ConnectWithSession(awsSession *session.Session)
	GetSession() *session.Session
	Send(queueUrl string, message string) error
}

type Client struct {
	cfg       *aws.Config
	sqsClient sqsiface.SQSAPI
	session   *session.Session
}

func New() *Client {
	cred := credentials.NewEnvCredentials()
	cfg := aws.NewConfig()
	cfg.WithRegion(os.Getenv(EnvRegion))
	cfg.WithCredentials(cred)

	return &Client{
		cfg: cfg,
	}
}

func (c *Client) Connect() error {
	awsSession, err := session.NewSession(c.cfg)
	if err != nil {
		return err
	}
	c.ConnectWithSession(awsSession)
	return nil
}

func (c *Client) ConnectWithSession(awsSession *session.Session) {
	c.session = awsSession
	c.sqsClient = sqs.New(c.session, c.cfg)
}

func (c *Client) GetSession() *session.Session {
	return c.session
}

func (c *Client) Send(queueUrl string, msg string) error {
	req, _ := c.sqsClient.SendMessageRequest(&sqs.SendMessageInput{
		QueueUrl:    aws.String(queueUrl),
		MessageBody: aws.String(msg),
		MessageGroupId: aws.String(groupId),
	})
	return req.Send()
}
