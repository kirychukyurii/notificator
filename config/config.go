package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/kirychukyurii/notificator/config/listeners"
	"github.com/kirychukyurii/notificator/config/notifiers"
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

type HttpServer struct {
	Bind string `yaml:"bind_address" json:"bind_address"`
	Root string `yaml:"root" json:"root"`
}

type Config struct {
	Timezone string `yaml:"timezone" json:"timezone"`

	Logger *Logger `yaml:"log" json:"log"`

	Manager    *Manager     `yaml:"manager" json:"manager"`
	Technicals []*Technical `yaml:"technicals" json:"technicals"`

	Start     []string      `yaml:"start" json:"start"`
	Stop      []string      `yaml:"stop" json:"stop"`
	GroupWait time.Duration `yaml:"group_wait" json:"group_wait"`

	HttpServer *HttpServer `yaml:"http" json:"http"`

	Listeners *listeners.Listeners `yaml:"listeners" json:"listeners"`
	Notifiers *notifiers.Notifiers `yaml:"notifiers" json:"notifiers"`
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
