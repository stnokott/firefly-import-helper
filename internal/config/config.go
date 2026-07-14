// Package config reads the configuration from an env file and provides the values in a central struct.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"net/url"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/kelseyhightower/envconfig"
	"github.com/stnokott/firefly-import-helper/internal/domain"
)

// Env contains the configuration data read from environment variables
type Env struct {
	TelegramBotToken   string `required:"true" envconfig:"TELEGRAM_BOT_TOKEN"`
	TelegramChatID     string `required:"true" envconfig:"TELEGRAM_CHAT_ID"`
	FireflyBaseURL     *URL   `required:"true" envconfig:"FIREFLY_BASE_URL"`
	FireflyAccessToken string `required:"true" envconfig:"FIREFLY_ACCESS_TOKEN"`
	LunchflowAPIKey    string `required:"true" envconfig:"LUNCHFLOW_API_KEY"`
}

// YAML contains the configuration data read from the config YAML file.
type YAML struct {
	Accounts Accounts `yaml:"accounts"`

	AccountsByBankID map[int]Account `yaml:"-"` // internally derived from Accounts
}

type Account struct {
	Name      string `yaml:"name"`
	BankID    int    `yaml:"bank_id"`
	FireflyID string `yaml:"firefly_id"`
	Ignore    bool   `yaml:"ignore"`
}

type Accounts []Account

func (a Accounts) Active() iter.Seq[Account] {
	return func(yield func(Account) bool) {
		for _, acc := range a {
			if !acc.Ignore && !yield(acc) {
				return
			}
		}
	}
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

func (cfg *YAML) validate() error {
	seenIDs := map[int]struct{}{}
	for acc := range cfg.Accounts.Active() {
		if acc.BankID == 0 {
			return fmt.Errorf(`account "%s": "bank_id" is required - re-initialize config if you accidentally deleted it`, acc.Name)
		}
		if acc.FireflyID == "" {
			return fmt.Errorf(`account "%s": set "firefly_id" or "ignore: true"`, acc.Name)
		}

		if _, seen := seenIDs[acc.BankID]; seen {
			return fmt.Errorf(`account "%s": duplicate "bank_id" %d - please re-initialize the config`, acc.Name, acc.BankID)
		}
		seenIDs[acc.BankID] = struct{}{}
	}
	return nil
}

// ValidateFireflyIDs ensures all Firefly account IDs in the config match actual IDs in the Firefly instance.
func (cfg *YAML) ValidateFireflyIDs(ffAccounts domain.FireflyAccounts) error {
	accountIDs := map[string]struct{}{}
	for acc := range ffAccounts.Active() {
		accountIDs[acc.ID] = struct{}{}
	}

	for acc := range cfg.Accounts.Active() {
		if _, exists := accountIDs[acc.FireflyID]; !exists {
			return fmt.Errorf(`account "%s": no active account with id "%s" found in Firefly`, acc.Name, acc.FireflyID)
		}
	}
	return nil
}

// ReadYAML reads yamlFile into a [YAML] instance.
func ReadYAML(yamlFile string) (*YAML, error) {
	cfg := new(YAML)
	if !Exists(yamlFile) {
		return nil, ErrConfigFileNotExist
	}

	if err := readYAML(yamlFile, cfg); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	cfg.AccountsByBankID = make(map[int]Account, len(cfg.Accounts))
	for _, acc := range cfg.Accounts {
		cfg.AccountsByBankID[acc.BankID] = acc
	}
	return cfg, nil
}

func readYAML(file string, cfg *YAML) error {
	yamlBytes, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("could not read config file %s: %w", file, err)
	}

	if err = yaml.Unmarshal(yamlBytes, cfg); err != nil {
		return fmt.Errorf("could not decode YAML in %s: %w", file, err)
	}
	return nil
}

var (
	ErrConfigFileNotExist = errors.New("config file does not exist")
	ErrConfigFileExist    = errors.New("config file already exists")
)

func ReadEnv() (*Env, error) {
	env := new(Env)
	if err := envconfig.Process("", env); err != nil {
		buf := bytes.NewBuffer(nil)
		_ = envconfig.Usagef("", env, buf, envconfig.DefaultTableFormat)
		return nil, fmt.Errorf("%w: %s", err, buf.String())
	}
	return env, nil
}

func Exists(file string) bool {
	_, err := os.Stat(file)
	if err == nil {
		return true
	}
	return !errors.Is(err, fs.ErrNotExist)
}

func WriteBootstrappedConfig(file string, accounts []domain.BankAccount) error {
	if Exists(file) {
		return ErrConfigFileExist
	}
	cfg := createBootstrappedConfig(accounts)

	data, err := yaml.MarshalWithOptions(
		cfg,
		yaml.WithComment(yamlCommentMap(cfg)),
		yaml.Indent(2),
		yaml.IndentSequence(true),
	)
	if err != nil {
		return fmt.Errorf("could not marshal YAML: %w", err)
	}
	if err = os.WriteFile(file, data, 0o644); err != nil {
		return fmt.Errorf("could not write to file %s: %w", file, err)
	}
	return nil
}

func createBootstrappedConfig(accs []domain.BankAccount) *YAML {
	transformed := make([]Account, len(accs))
	for i, acc := range accs {
		transformed[i] = Account{
			BankID: acc.ID,
			Name:   fmt.Sprintf("%s (%s)", acc.Name, acc.Institution),
		}
	}
	return &YAML{
		Accounts: transformed,
	}
}

func yamlCommentMap(cfg *YAML) yaml.CommentMap {
	cm := yaml.CommentMap{
		"$.accounts": []*yaml.Comment{
			yaml.HeadComment(
				" This is the initial bootstrapped config.",
				"",
				` Set "firefly_id" to control which Firefly account a transaction from`,
				" a bank account gets added to.",
				"",
				` Set "ignore" to true to exclude a bank account from processing.`,
				"",
				` The "name" field is not used, feel free to modify it.`,
			),
		},
	}
	// yaml library has a limitation which doesn't allow comments on children of wildcard "[*]" selectors,
	// so we need to generate a dedicated comment for each list item instead of using the wildcard.
	for i := range cfg.Accounts {
		cm[fmt.Sprintf("$.accounts[%d].bank_id", i)] = []*yaml.Comment{
			yaml.LineComment(" DO NOT MODIFY"),
		}
	}
	return cm
}
