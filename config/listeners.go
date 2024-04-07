package config

var (
	DefaultTelegramConfig = TelegramConfig{}
	DefaultSlackConfig    = SlackConfig{}
	DefaultSkypeConfig    = SkypeConfig{}
)

type Listeners struct {
	Start     []string `yaml:"start" json:"start"`
	Stop      []string `yaml:"stop" json:"stop"`
	GroupWait string   `yaml:"group_wait" json:"group_wait"`

	TelegramConfigs []*TelegramConfig `yaml:"telegram_configs" json:"telegram_configs"`
	SlackConfigs    []*SlackConfig    `yaml:"slack_configs" json:"slack_configs"`
	SkypeConfigs    []*SkypeConfig    `yaml:"skype_configs" json:"skype_configs"`
}

type TelegramConfig struct {
	Phone            string `yaml:"phone" json:"phone"`
	AppID            int    `yaml:"app_id" json:"app_id"`
	AppHash          string `yaml:"app_hash" json:"app_hash"`
	FillPeersOnStart bool   `yaml:"fill_peers_on_start" json:"fill_peers_on_start" `
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *TelegramConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultTelegramConfig
	type plain TelegramConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return nil
}

type SlackConfig struct{}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *SlackConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultSlackConfig
	type plain SlackConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return nil
}

type SkypeConfig struct{}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *SkypeConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultSkypeConfig
	type plain SkypeConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return nil
}
