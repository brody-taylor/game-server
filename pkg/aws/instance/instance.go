package instance

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

const (
	InstanceRunningState      = "running"
	InstancePendingState      = "pending"
	InstanceStoppingState     = "stopping"
	InstanceStoppedState      = "stopped"
	InstanceShuttingDownState = "shutting-down"
	InstanceTerminatedState   = "terminated"

	EnvRegion = "AWS_REGION"
)

// Ensure Client implements ClientIFace
var _ ClientIFace = (*Client)(nil)

type ClientIFace interface {
	Connect() error
	ConnectWithSession(awsSession *session.Session)
	GetSession() *session.Session
	GetInstanceState(id string) (state string, err error)
	GetInstanceAddress(id string) (address string, err error)
	StartInstance(id string) error
}

type Client struct {
	cfg            *aws.Config
	instanceClient ec2iface.EC2API
	session        *session.Session
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
	c.instanceClient = ec2.New(c.session, c.cfg)
}

func (c *Client) GetSession() *session.Session {
	return c.session
}

func (c *Client) GetInstanceState(id string) (string, error) {
	in := &ec2.DescribeInstanceStatusInput{
		IncludeAllInstances: aws.Bool(true),
		InstanceIds: []*string{
			aws.String(id),
		},
	}

	out, err := c.instanceClient.DescribeInstanceStatus(in)
	if err != nil {
		return "", err
	} else if numInstances := len(out.InstanceStatuses); numInstances != 1 {
		return "", fmt.Errorf("invalid number of instances found: [%v]", numInstances)
	}
	state := *out.InstanceStatuses[0].InstanceState.Name

	return state, nil
}

func (c *Client) GetInstanceAddress(id string) (string, error) {
	in := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(id)},
	}

	// TODO: also returns instance state, consolidate w/ GetInstanceState into one call
	out, err := c.instanceClient.DescribeInstances(in)
	if err != nil {
		return "", err
	} else if numInstances := len(out.Reservations[0].Instances); numInstances != 1 {
		return "", fmt.Errorf("invalid number of instances found: [%v]", numInstances)
	}
	ip := *out.Reservations[0].Instances[0].PublicDnsName

	return ip, nil
}

func (c *Client) StartInstance(id string) error {
	in := &ec2.StartInstancesInput{
		InstanceIds: []*string{
			aws.String(id),
		},
	}

	_, err := c.instanceClient.StartInstances(in)
	return err
}
