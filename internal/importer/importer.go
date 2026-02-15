package importer

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/log"
	"github.com/stnokott/firefly-import-helper/internal/telegram"
)

type Importer struct {
	t telegram.Bot
}

func New(t telegram.Bot) *Importer {
	return &Importer{
		t: t,
	}
}

const importTimeout = time.Duration(5 * time.Minute)

var logger = log.For("importer")

func (im *Importer) Run(ctx context.Context, importerURL string, importerSecret string, fireflyAccessToken string, configPath string) error {
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

	logger.Debugf("executing request with timeout %s", importTimeout.String())
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
	logger.Info("import finished")
	im.analyzeAutoimportResponse(ctx, resp)
	return nil
}

func createAutouploadRequest(ctx context.Context, reqURL string, configPath string) (*http.Request, error) {
	_, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file %s does not exist", configPath)
		}
		return nil, fmt.Errorf("cannot access config file %s: %w", configPath, err)
	}

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

// matches an importer response line with details about a transaction import.
//
// If the capturing group contains a value, it indicates an error for this transaction.
var rxImportRespLine = regexp.MustCompile(`(?m)^.+ Import index \d+: (\[a\d+\]: )?(.+)$`)

// analyzeAutoimportResponse attempts to parse the response body to determine and
// log the import results.
func (im *Importer) analyzeAutoimportResponse(ctx context.Context, resp *http.Response) {
	r := bufio.NewReader(resp.Body)
	success := 0
	errors := 0
	for {
		line, err := r.ReadString('\n')
		matches := rxImportRespLine.FindStringSubmatch(line)
		if len(matches) > 1 {
			isSuccess := matches[1] == ""
			if isSuccess {
				success++
			} else {
				logger.Warnf("possible importer error: %s", strings.TrimSpace(matches[2]))
				errors++
			}
		}
		if err != nil {
			break
		}
	}

	logger.Infof(">> %d successful, %d errors", success, errors)
	if err := im.t.SendImportFinished(ctx, success, errors); err != nil {
		logger.Warnf("could not send import finished message: %v", err)
	}
}
