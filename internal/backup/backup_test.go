package backup

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"game-server/internal/config"
	"game-server/internal/testing/mockserver"
	"game-server/pkg/aws/s3"
)

func Test_Client_DoBackup(t *testing.T) {
	bucketName := "save-bucket"
	t.Setenv(EnvGameSaveBucket, bucketName)

	// Override save frequency to ensure backup occurs
	defer func(origFreq time.Duration) {
		Frequency = origFreq
	}(Frequency)
	Frequency = time.Duration(0)

	mockCfg := mockserver.GetConfig(t)

	// Setup mock S3 client
	expFolderName := time.Now().Format(dateFolderFormat)
	mockS3Client := new(s3.MockClient)
	mockS3Client.On(s3.ConnectMethod).Return(nil)
	mockS3Client.On(s3.GetFoldersMethod, bucketName, 2).Return(nil, nil)
	for _, saveFile := range mockserver.SaveFilePaths {
		mockS3Client.On(s3.PutMethod, mock.Anything, bucketName, path.Join(mockserver.GameName, expFolderName, saveFile)).Return(nil).Once()
	}

	c := Client{
		cfg:      mockCfg,
		logger:   config.NewTestLogger(),
		s3Bucket: bucketName,
		s3Client: mockS3Client,
	}

	err := c.DoBackup()

	require.NoError(t, err)

	// Ensure every save file was uploaded
	for _, saveFile := range mockserver.SaveFilePaths {
		mockS3Client.AssertCalled(t, s3.PutMethod, mock.Anything, bucketName, path.Join(mockserver.GameName, expFolderName, saveFile))
	}
}
