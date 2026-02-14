package importer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/log"
)

const importTimeout = 5 * time.Minute

var logger = log.For("importer")

func Run(ctx context.Context, importerURL string, importerSecret string, fireflyAccessToken string, configPath string) error {
	logger.Infof("running with config '%s'", configPath)

	ctx, cancel := context.WithTimeout(ctx, importTimeout)
	defer cancel()

	u, err := url.Parse(importerURL)
	if err != nil {
		return fmt.Errorf("invalid importer URL: %w", err)
	}
	u = u.JoinPath("autoupload")
	q := u.Query()
	q.Set("secret", importerSecret)
	u.RawQuery = q.Encode()

	req, err := createAutouploadRequest(ctx, u.String(), configPath)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+fireflyAccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not execute request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("invalid status code %d, could not read response body", resp.StatusCode)
		}
		return fmt.Errorf("invalid status code %d, got response: '%s'", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func createAutouploadRequest(ctx context.Context, reqURL string, configPath string) (*http.Request, error) {
	form := new(bytes.Buffer)
	writer := multipart.NewWriter(form)
	fw, err := writer.CreateFormFile("json", filepath.Base(configPath))
	if err != nil {
		return nil, fmt.Errorf("could not create form file: %w", err)
	}
	fd, err := os.OpenFile(configPath, os.O_RDONLY, 0x664)
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}
	defer fd.Close()
	_, err = io.Copy(fw, fd)
	if err != nil {
		return nil, fmt.Errorf("could not write to form: %w", err)
	}
	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("could not close form writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, form)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}
