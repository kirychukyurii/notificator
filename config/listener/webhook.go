package listener

var DefaultWebhookConfig = WebhookConfig{}

type WebhookResponseMap struct {
	Message string `yaml:"message" json:"message"`
	From    string `yaml:"from" json:"from"`
	Chat    string `yaml:"chat" json:"chat"`
}

type WebhookConfig struct {
	Name        string             `yaml:"path" json:"path"`
	Token       string             `yaml:"token" json:"token"`
	ResponseMap WebhookResponseMap `yaml:"response_map" json:"response_map"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *WebhookConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultWebhookConfig
	type plain WebhookConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return nil
}
