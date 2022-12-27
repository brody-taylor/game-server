package backup

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap"

	"game-server/internal/config"
	"game-server/pkg/aws/s3"
	customerrors "game-server/pkg/errors"
)

const (
	EnvGameSaveBucket = "GAME_SAVE_BUCKET"

	loggerName = "save-backup"

	dateFolderFormat = "2006-01-02"
)

var (
	Frequency = time.Hour * 24 // Default backup daily
)

type Client struct {
	cfg    *config.Config
	logger *zap.Logger

	s3Bucket string
	s3Client s3.ClientIFace
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg:      cfg,
		logger:   cfg.Logger.Named(loggerName),
		s3Client: s3.New(),
	}
}

func (c *Client) DoBackup() error {
	if err := c.start(); err != nil {
		return err
	}

	// Get dates of last S3 backup for all games
	s3Saves, err := c.getPrevSaveDates()
	if err != nil {
		return err
	}

	// Do backup for each game
	var multiErr error
	for _, game := range c.cfg.GetGameNames() {
		gameCfg, _ := c.cfg.GetGameConfig(game)
		if err := c.backupGame(gameCfg, s3Saves[game]); err != nil {
			multiErr = multierr.Append(multiErr, err)
		}
	}
	return multiErr
}

func (c *Client) start() error {
	// Connect AWS
	if err := c.s3Client.Connect(); err != nil {
		return err
	}

	// Get env variables
	bucket, ok := os.LookupEnv(EnvGameSaveBucket)
	if !ok {
		return customerrors.MissingEnvErr{}
	}
	c.s3Bucket = bucket

	return nil
}

func (c *Client) getPrevSaveDates() (map[string]time.Time, error) {
	// Get all folders in the game save bucket
	saveFolders, err := c.s3Client.GetFolders(c.s3Bucket, 2)
	if err != nil {
		return nil, err
	}

	// Build map of most recent save dates for each game
	lastSaveDates := make(map[string]time.Time)
	for _, saveFolder := range saveFolders {
		folders := strings.Split(saveFolder, s3.Delimiter)

		// Validate game name and add to map
		gameName := folders[0]
		if _, ok := c.cfg.GetGameConfig(gameName); ok && len(folders) > 1 {
			datedFolder := folders[1]

			// Parse folder name into date
			saveDate, err := time.Parse(dateFolderFormat, datedFolder)
			if err != nil {
				continue
			}

			// Add if most recent
			if mostRecent, ok := lastSaveDates[gameName]; ok {
				if mostRecent.Before(saveDate) {
					lastSaveDates[gameName] = saveDate
				}
			} else {
				lastSaveDates[gameName] = saveDate
			}
		}
	}

	return lastSaveDates, nil
}

func (c *Client) backupGame(gameCfg *config.GameConfig, lastSave time.Time) error {
	saveFiles, lastMod, err := getSaveFiles(gameCfg)
	if err != nil {
		return fmt.Errorf("could not get %s save files: %w", gameCfg.Name, err)
	}
	defer closeSaveFiles(saveFiles)

	// Skip game if last backup was recent enough
	if lastMod.Before(lastSave.Add(Frequency)) {
		c.logger.Info("no backup required", zap.String("game", gameCfg.Name))
		return nil
	}

	c.logger.Info(
		"backing up save",
		zap.String("game", gameCfg.Name),
		zap.Time("previous", lastSave),
		zap.Time("modified", lastMod),
	)

	// Add each file to S3
	var multiErr error
	dateFolder := time.Now().Format(dateFolderFormat)
	for filePath, saveFile := range saveFiles {
		key := path.Join(gameCfg.Name, dateFolder, filePath)
		if err := c.s3Client.Put(saveFile, c.s3Bucket, key); err != nil {
			multiErr = multierr.Append(multiErr, err)
		}
	}
	return multiErr
}

func getSaveFiles(cfg *config.GameConfig) (saveFiles map[string]io.ReadSeekCloser, lastModified time.Time, err error) {
	saveFiles = make(map[string]io.ReadSeekCloser)

	// If failure, ensure all files are closed
	defer func() {
		if err != nil {
			closeSaveFiles(saveFiles)
			saveFiles = nil
		}
	}()

	// Handler for each save file
	var handler fs.WalkDirFunc = func(filePath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		f, err := os.Open(filePath)
		if err != nil {
			return err
		}

		// Get last modified time from file metadata
		fInfo, err := d.Info()
		if err != nil {
			f.Close()
			return err
		}
		if lastModified.Before(fInfo.ModTime()) {
			lastModified = fInfo.ModTime()
		}

		saveFilePath := strings.ReplaceAll(path.Clean(filePath), "\\", "/")
		saveFilePath = strings.TrimPrefix(saveFilePath, path.Clean(cfg.WorkingDir))
		saveFiles[saveFilePath] = f
		return nil
	}

	// Call handler for each save file
	for _, saveFile := range cfg.SaveFiles {
		filePath := path.Join(cfg.WorkingDir, saveFile)
		if err = filepath.WalkDir(filePath, handler); err != nil {
			break
		}
	}

	return saveFiles, lastModified, err
}

func closeSaveFiles(saveFiles map[string]io.ReadSeekCloser) {
	for _, f := range saveFiles {
		f.Close()
	}
}
