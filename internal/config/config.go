package config

import (
	"github.com/spf13/viper"
)

// Config stores all configuration of the application.
// The values are read by viper from a config file or environment variables.
type Config struct {
	Slack  SlackConfig  `mapstructure:"slack"`
	Gemini GeminiConfig `mapstructure:"gemini"`
}

// SlackConfig stores the configuration for the Slack service.
type SlackConfig struct {
	Token          string `mapstructure:"token"`
	SigningSecret string `mapstructure:"signing_secret"`
}

// GeminiConfig stores the configuration for the Gemini service.
type GeminiConfig struct {
	APIKey string `mapstructure:"api_key"`
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
