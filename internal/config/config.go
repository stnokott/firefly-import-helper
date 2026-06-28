// Package config reads the configuration from an env file and provides the values in a central struct.
package config

import (
	"bytes"
	"fmt"
	"net/url"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Spec contains the configuration data read from the env file.
type Spec struct {
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

// C is an instance of [Spec], allowing you to read config data.
//
// Make sure to call [Read] once before accessing it.
var C Spec

// Read reads the contents of "file", expecting an env-file like structure.
// TODO: return config struct instead of global singleton
func Read(file string) error {
	if err := godotenv.Load(file); err != nil {
		return fmt.Errorf("could not read env file '%s': %w", file, err)
	}
	if err := envconfig.Process("", &C); err != nil {
		buf := bytes.NewBuffer(nil)
		_ = envconfig.Usagef("", &C, buf, envconfig.DefaultTableFormat)
		return fmt.Errorf("%w: %s", err, buf.String())
	}
	return nil
}
