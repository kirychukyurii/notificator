package listeners

var DefaultTelegramConfig = TelegramConfig{}

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
