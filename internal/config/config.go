// Package config reads the configuration from an env file and provides the values in a central struct.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"slices"

	"github.com/goccy/go-yaml"
	"github.com/kelseyhightower/envconfig"
	"github.com/stnokott/firefly-import-helper/internal/domain"
)

// Config contains the configuration data read from environment variables.
type Config struct {
	Env      `yaml:"-"` // filled from environment variables
	Accounts Accounts   `yaml:"accounts"`

	AccountsByBankID map[int]*Account `yaml:"-"` // filled from Accounts
}

type Env struct {
	TelegramBotToken   string `required:"true" envconfig:"TELEGRAM_BOT_TOKEN"`
	TelegramChatID     string `required:"true" envconfig:"TELEGRAM_CHAT_ID"`
	FireflyBaseURL     *URL   `required:"true" envconfig:"FIREFLY_BASE_URL"`
	FireflyAccessToken string `required:"true" envconfig:"FIREFLY_ACCESS_TOKEN"`
	LunchflowAPIKey    string `required:"true" envconfig:"LUNCHFLOW_API_KEY"`
}

type Account struct {
	Name      string `yaml:"name"`
	BankID    int    `yaml:"bank_id"`
	FireflyID string `yaml:"firefly_id"`
	Ignore    bool   `yaml:"ignore"`
}

type Accounts []*Account

func (a Accounts) Active() Accounts {
	dst := make(Accounts, len(a))
	copy(dst, a)
	return slices.DeleteFunc(dst, func(acc *Account) bool {
		return acc.Ignore
	})
}

type URL struct {
	url.URL
}

func (u *URL) Decode(v string) error {
	parsed, err := url.Parse(v)
	if err != nil {
		return err
	}
	*u = URL{
		URL: *parsed,
	}
	return nil
}

func (c *Config) validate() error {
	seenIDs := map[int]struct{}{}
	for _, acc := range c.Accounts.Active() {
		if acc.BankID == 0 {
			return fmt.Errorf(`account "%s": "bank_id" is required - re-initialize config if you accidentally deleted it`, acc.Name)
		}
		if acc.FireflyID == "" {
			return fmt.Errorf(`account "%s": set "firefly_id" or ignore this account`, acc.Name)
		}

		if _, seen := seenIDs[acc.BankID]; seen {
			return fmt.Errorf(`account "%s": found duplicate "bank_id" %d - please re-initialize the config`, acc.Name, acc.BankID)
		}
		seenIDs[acc.BankID] = struct{}{}
	}
	return nil
}

// ValidateFireflyIDs ensures all Firefly account IDs in the config match actual IDs in the Firefly instance.
func (c *Config) ValidateFireflyIDs(ffAccounts []domain.FireflyAccount) error {
	accountIDs := map[string]struct{}{}
	for _, acc := range ffAccounts {
		accountIDs[acc.ID] = struct{}{}
	}

	for _, acc := range c.Accounts.Active() {
		if _, exists := accountIDs[acc.FireflyID]; !exists {
			return fmt.Errorf(`account "%s": "firefly_id" "%s" not found in Firefly`, acc.Name, acc.FireflyID)
		}
	}
	return nil
}

// Read reads config from environment variables and a config file
func Read(yamlFile string, allowEmptyYAML bool) (*Config, error) {
	env, err := readEnv()
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		Env: *env,
	}
	if !fileExists(yamlFile) {
		if allowEmptyYAML {
			return cfg, nil
		}
		return nil, ErrConfigFileNotExist
	}

	if err = readYAML(yamlFile, cfg); err != nil {
		return nil, err
	}

	if err = cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	cfg.AccountsByBankID = make(map[int]*Account, len(cfg.Accounts))
	for _, acc := range cfg.Accounts {
		cfg.AccountsByBankID[acc.BankID] = acc
	}
	return cfg, nil
}

var ErrConfigFileNotExist = errors.New("config file does not exist")

func readEnv() (*Env, error) {
	env := new(Env)
	if err := envconfig.Process("", env); err != nil {
		buf := bytes.NewBuffer(nil)
		_ = envconfig.Usagef("", env, buf, envconfig.DefaultTableFormat)
		return nil, fmt.Errorf("%w: %s", err, buf.String())
	}
	return env, nil
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	if err == nil {
		return true
	}
	return !errors.Is(err, fs.ErrNotExist)
}

func readYAML(file string, cfg *Config) error {
	yamlBytes, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("could not read config file %s: %w", file, err)
	}

	if err = yaml.Unmarshal(yamlBytes, cfg); err != nil {
		return fmt.Errorf("could not decode YAML in %s: %w", file, err)
	}
	return nil
}

func WriteBootstrappedConfig(file string, accounts []domain.BankAccount) error {
	if fileExists(file) {
		return errors.New("config file already exists")
	}
	cfg := createBootstrappedConfig(accounts)
	data, err := yaml.MarshalWithOptions(cfg, yaml.WithComment(yaml.CommentMap{
		"$.accounts": []*yaml.Comment{
			{
				Position: 0,
				Texts: []string{
					" This is the initial bootstrapped config.",
					"",
					` Set "firefly_id" to control which Firefly account a transaction from`,
					" a bank account gets added to.",
					"",
					` Set "ignore" to true to exclude a bank account from processing.`,
					"",
					` The "name" field only exists for identification, feel free to modify it.`,
				},
			},
		},
	}), yaml.Indent(2), yaml.IndentSequence(true))
	if err != nil {
		return fmt.Errorf("could not marshal YAML: %w", err)
	}
	if err = os.WriteFile(file, data, 0o644); err != nil {
		return fmt.Errorf("could not write to file %s: %w", file, err)
	}
	return nil
}

func createBootstrappedConfig(accs []domain.BankAccount) *Config {
	transformed := make([]*Account, len(accs))
	for i, acc := range accs {
		transformed[i] = &Account{
			BankID: acc.ID,
			Name:   fmt.Sprintf("%s(%s)", acc.Name, acc.Institution),
		}
	}
	return &Config{
		Accounts: transformed,
	}
}
