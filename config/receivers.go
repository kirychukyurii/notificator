package config

var (
	DefaultWebhookConfig = WebhookConfig{}
	DefaultWebitelConfig = WebitelConfig{
		Authorization: &Authorization{
			Header: "X-Webitel-Access",
		},
	}
)

type Receivers struct {
	WebhookConfigs []*WebhookConfig `yaml:"webhook_configs" json:"webhook_configs"`
	WebitelConfigs []*WebitelConfig `yaml:"webitel_configs" json:"webitel_configs"`
}

type Authorization struct {
	Header string `yaml:"header,omitempty" json:"header,omitempty"`
	Value  string `yaml:"value,omitempty" json:"value,omitempty"`
}

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

type WebitelConfig struct {
	URL           string         `yaml:"url" json:"url"`
	Authorization *Authorization `yaml:"authorization,omitempty" json:"authorization,omitempty"`

	QueueID int `yaml:"queue_id,omitempty" json:"queue_id,omitempty"`
	TypeID  int `yaml:"type_id,omitempty" json:"type_id,omitempty"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *WebitelConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultWebitelConfig
	type plain WebitelConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return nil
}
