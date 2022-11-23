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
	GetInstanceState(id string) (state string, err error)
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
	var err error
	c.session, err = session.NewSession(c.cfg)
	if err != nil {
		return err
	}

	c.instanceClient = ec2.New(c.session, c.cfg)
	return nil
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

func (c *Client) StartInstance(id string) error {
	in := &ec2.StartInstancesInput{
		InstanceIds: []*string{
			aws.String(id),
		},
	}

	_, err := c.instanceClient.StartInstances(in)
	return err
}
