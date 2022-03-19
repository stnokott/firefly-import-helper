package autoimport

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const jsonDir = "/configs"

type Manager struct {
	url    string
	client *http.Client
}

func NewManager(autoImporterUrl string, autoImporterPort uint, secret string) (*Manager, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	return &Manager{
		url:    fmt.Sprintf("%s:%d/autoupload?secret=%s", autoImporterUrl, autoImporterPort, secret),
		client: client,
	}, nil
}

func (m *Manager) Import(jsonFilepath string) error {
	var bodyBuf bytes.Buffer
	bodyWriter := multipart.NewWriter(&bodyBuf)
	fileReader, err := os.Open(jsonFilepath)
	if err != nil {
		return err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer fileReader.Close()

	fileWriter, err := bodyWriter.CreateFormFile("json", fileReader.Name())
	if err != nil {
		return err
	}
	if _, err = io.Copy(fileWriter, fileReader); err != nil {
		return err
	}
	//goland:noinspection GoUnhandledErrorResult
	bodyWriter.Close()

	r, err := http.NewRequest(http.MethodPost, m.url, &bodyBuf)
	if err != nil {
		return err
	}
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", bodyWriter.FormDataContentType())

	resp, err := m.client.Do(r)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		//goland:noinspection GoUnhandledErrorResult
		defer resp.Body.Close()
		respBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.New("unknown error occured")
		} else {
			return errors.New(string(respBytes))
		}
	}
	return nil
}

func (m *Manager) GetJsonFilePaths() ([]string, error) {
	files, err := os.ReadDir(jsonDir)
	if err != nil {
		return nil, err
	}
	var filepaths []string
	for _, entry := range files {
		if !entry.IsDir() && strings.Contains(entry.Name(), ".json") {
			fullPath := filepath.Join(jsonDir, entry.Name())
			filepaths = append(filepaths, fullPath)
		}
	}
	return filepaths, nil
}
