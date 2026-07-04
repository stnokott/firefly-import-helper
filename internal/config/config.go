// Package config reads the configuration from an env file and provides the values in a central struct.
package config

import (
	"bytes"
	"fmt"
	"net/url"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config contains the configuration data read from environment variables.
type Config struct {
	TelegramBotToken   string `required:"true" envconfig:"TELEGRAM_BOT_TOKEN"`
	TelegramChatID     string `required:"true" envconfig:"TELEGRAM_CHAT_ID"`
	FireflyBaseURL     *URL   `required:"true" envconfig:"FIREFLY_BASE_URL"`
	FireflyAccessToken string `required:"true" envconfig:"FIREFLY_ACCESS_TOKEN"`
	LunchflowAPIKey    string `required:"true" envconfig:"LUNCHFLOW_API_KEY"`
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

// Read reads the contents of "file", expecting an env-file like structure.
func Read(file string) (*Config, error) {
	if err := godotenv.Load(file); err != nil {
		return nil, fmt.Errorf("could not read env file '%s': %w", file, err)
	}
	c := new(Config)
	if err := envconfig.Process("", c); err != nil {
		buf := bytes.NewBuffer(nil)
		_ = envconfig.Usagef("", &c, buf, envconfig.DefaultTableFormat)
		return nil, fmt.Errorf("%w: %s", err, buf.String())
	}
	return c, nil
}
