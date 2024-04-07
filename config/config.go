package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Logger struct {
	Level string `yaml:"level" json:"level"`
}

type Manager struct {
	BotID  string `yaml:"bot_id" json:"bot_id"`
	ChatID int64  `yaml:"chat_id" json:"chat_id"`
}

type Technical struct {
	Name   string `yaml:"name" json:"name"`
	Phone  string `yaml:"phone" json:"phone"`
	OnDuty bool
}

type Config struct {
	Timezone string `yaml:"timezone" json:"timezone"`

	Logger *Logger `yaml:"log" json:"log"`

	Manager    *Manager     `yaml:"manager" json:"manager"`
	Technicals []*Technical `yaml:"technicals" json:"technicals"`

	Listeners *Listeners `yaml:"listeners" json:"listeners"`
	Receivers *Receivers `yaml:"receivers" json:"receivers"`
}

func New(filename string) (*Config, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(content, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
