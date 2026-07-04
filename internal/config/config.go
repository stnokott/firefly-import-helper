// Package config reads the configuration from an env file and provides the values in a central struct.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/kelseyhightower/envconfig"
	"github.com/stnokott/firefly-import-helper/internal/domain"
)

// Config contains the configuration data read from environment variables.
type Config struct {
	Env      `yaml:"-"` // filled from environment variables
	Accounts []Account  `yaml:"accounts"`
}

type Env struct {
	TelegramBotToken   string `required:"true" envconfig:"TELEGRAM_BOT_TOKEN"`
	TelegramChatID     string `required:"true" envconfig:"TELEGRAM_CHAT_ID"`
	FireflyBaseURL     *URL   `required:"true" envconfig:"FIREFLY_BASE_URL"`
	FireflyAccessToken string `required:"true" envconfig:"FIREFLY_ACCESS_TOKEN"`
	LunchflowAPIKey    string `required:"true" envconfig:"LUNCHFLOW_API_KEY"`
}

type Account struct {
	Name      string               `yaml:"name"`
	BankID    domain.BankAccountID `yaml:"bank_id"`
	FireflyID string               `yaml:"firefly_id"`
	Ignore    bool                 `yaml:"ignore"`
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
	for _, acc := range c.Accounts {
		if acc.BankID == 0 {
			return fmt.Errorf(`account "%s": "bank_id" is required - re-initialize config if you accidentally deleted it`, acc.Name)
		}
		if acc.FireflyID == "" && !acc.Ignore {
			return fmt.Errorf(`account "%s": set "firefly_id" or ignore this account`, acc.Name)
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
	transformed := make([]Account, len(accs))
	for i, acc := range accs {
		transformed[i] = Account{
			BankID: acc.ID,
			Name:   fmt.Sprintf("%s(%s)", acc.Name, acc.Institution),
		}
	}
	return &Config{
		Accounts: transformed,
	}
}
