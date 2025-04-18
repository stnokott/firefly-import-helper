// Package autoimport manages scheduling and basic API interfacing
package autoimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const jsonDir = "/configs"

// Manager handles imports and all handling of the results
type Manager struct {
	url    string
	client *http.Client
}

// NewManager creates a new manager instance
func NewManager(autoImporterURL string, autoImporterPort int, secret string) (*Manager, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	return &Manager{
		url:    fmt.Sprintf("%s:%d/autoupload?secret=%s", autoImporterURL, autoImporterPort, secret),
		client: client,
	}, nil
}

// Import starts the import process, using the provided JSON as config
func (m *Manager) Import(jsonFilepath string) (err error) {
	var (
		bodyBuf    bytes.Buffer
		fileReader *os.File
	)
	bodyWriter := multipart.NewWriter(&bodyBuf)
	fileReader, err = os.Open(jsonFilepath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, fileReader.Close())
	}()

	var fileWriter io.Writer
	fileWriter, err = bodyWriter.CreateFormFile("json", fileReader.Name())
	if err != nil {
		return
	}
	if _, err = io.Copy(fileWriter, fileReader); err != nil {
		return
	}
	if err = errors.Join(err, bodyWriter.Close()); err != nil {
		return
	}

	var r *http.Request
	r, err = http.NewRequest(http.MethodPost, m.url, &bodyBuf)
	if err != nil {
		return
	}
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", bodyWriter.FormDataContentType())

	var resp *http.Response
	resp, err = m.client.Do(r)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		// handle erroneous response
		defer func() {
			err = errors.Join(err, resp.Body.Close())
		}()
		var respBytes []byte
		respBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			err = errors.New("unknown error occured")
		} else {
			err = errors.New(string(respBytes))
		}
		return
	}
	if err = updateJSONDates(jsonFilepath); err != nil {
		return
	}
	return
}

// GetJSONFilePaths returns a slice of filepaths for config files.
// Each of these can be passed as an argument to Import()
func (m *Manager) GetJSONFilePaths() ([]string, error) {
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
	if len(filepaths) == 0 {
		return nil, fmt.Errorf("did not detect any configuration files, please check %s folder", jsonDir)
	}

	return filepaths, nil
}

type configJSON struct {
	Version                     int               `json:"version"`
	Source                      string            `json:"source"`
	CreatedAt                   string            `json:"created_at"`
	Date                        string            `json:"date"`
	DefaultAccount              int               `json:"default_account"`
	Delimiter                   string            `json:"delimiter"`
	Headers                     bool              `json:"headers"`
	Rules                       bool              `json:"rules"`
	SkipForm                    bool              `json:"skip_form"`
	AddImportTag                bool              `json:"add_import_tag"`
	Roles                       []any             `json:"roles"`
	DoMapping                   []any             `json:"do_mapping"`
	Mapping                     []any             `json:"mapping"`
	DuplicateDetectionMethod    string            `json:"duplicate_detection_method"`
	IgnoreDuplicateLines        bool              `json:"ignore_duplicate_lines"`
	IgnoreDuplicateTransactions bool              `json:"ignore_duplicate_transactions"`
	UniqueColumnIndex           int               `json:"unique_column_index"`
	UniqueColumnType            string            `json:"unique_column_type"`
	Flow                        string            `json:"flow"`
	Identifier                  string            `json:"identifier"`
	Connection                  string            `json:"connection"`
	IgnoreSpectreCategories     bool              `json:"ignore_spectre_categories"`
	MapAllData                  bool              `json:"map_all_data"`
	Accounts                    map[string]int    `json:"accounts"`
	DateRange                   string            `json:"date_range"`
	DateRangeNumber             int               `json:"date_range_number"`
	DateRangeUnit               string            `json:"date_range_unit"`
	DateNotBefore               string            `json:"date_not_before"`
	DateNotAfter                string            `json:"date_not_after"`
	NordigenCountry             string            `json:"nordigen_country"`
	NordigenBank                string            `json:"nordigen_bank"`
	NordigenRequisitions        map[string]string `json:"nordigen_requisitions"`
	NordigenMaxDays             string            `json:"nordigen_max_days"`
	Conversion                  bool              `json:"conversion"`
}

func updateJSONDates(jsonPath string) error {
	body, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}

	var config configJSON
	if err := json.Unmarshal(body, &config); err != nil {
		return err
	}
	config.DateNotBefore = time.Now().AddDate(0, 0, -1).Format(time.DateOnly)
	b, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(jsonPath, b, 0600)
}
