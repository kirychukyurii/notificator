package listeners

var DefaultSlackConfig = SlackConfig{}

type SlackConfig struct {
	AppToken string `yaml:"app_token" json:"app_token"`
	BotToken string `yaml:"bot_token" json:"bot_token"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *SlackConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultSlackConfig
	type plain SlackConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return nil
}
