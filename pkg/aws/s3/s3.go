package s3

import (
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

const (
	Delimiter = "/"
)

// Ensure Client implements ClientIFace
var _ ClientIFace = (*Client)(nil)

type ClientIFace interface {
	Connect() error
	ConnectWithSession(awsSession *session.Session)
	GetSession() *session.Session
	GetFolders(bucket string, depth int) ([]string, error)
	Put(file io.ReadSeeker, bucket string, key string) error
}

type Client struct {
	cfg      *aws.Config
	s3Client s3iface.S3API
	session  *session.Session
}

func New() *Client {
	cfg := aws.NewConfig()
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
	c.s3Client = s3.New(c.session, c.cfg)
}

func (c *Client) GetSession() *session.Session {
	return c.session
}

func (c *Client) GetFolders(bucket string, depth int) ([]string, error) {
	if depth < 1 {
		return nil, fmt.Errorf("subdirectory depth must be at least 1")
	}

	req := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}
	out, err := c.s3Client.ListObjectsV2(req)
	if err != nil {
		return nil, err
	}

	// Build list of subdirectories up to specified depth
	var keys []string
	keySet := make(map[string]struct{})
	for _, file := range out.Contents {
		subDir := *file.Key

		// Skip files, a folder will end with delimiter
		if !strings.HasSuffix(subDir, Delimiter) {
			continue
		}

		// Remove path below max depth
		if folders := strings.Split(subDir, Delimiter); len(folders) > depth {
			subDir = strings.Join(folders[:depth-1], Delimiter)
		}

		if _, ok := keySet[subDir]; !ok {
			keys = append(keys, subDir)
			keySet[subDir] = struct{}{}
		}
	}

	return keys, nil
}

func (c *Client) Put(file io.ReadSeeker, bucket string, key string) error {
	//req, _ := c.s3Uploader.Upload()
	req, _ := c.s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	return req.Send()
}
