package notifiers

var DefaultWebhookConfig = WebhookConfig{}

type WebhookConfig struct {
	URL           string         `yaml:"url" json:"url"`
	Authorization *Authorization `yaml:"authorization,omitempty" json:"authorization,omitempty"`
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
