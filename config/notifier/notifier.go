package notifier

type Notifiers struct {
	StdOut bool `yaml:"stdout" json:"stdout"`

	WebhookConfigs []*WebhookConfig `yaml:"webhook_configs" json:"webhook_configs"`
	WebitelConfigs []*WebitelConfig `yaml:"webitel_configs" json:"webitel_configs"`
}

type Authorization struct {
	Header string `yaml:"header,omitempty" json:"header,omitempty"`
	Value  string `yaml:"value,omitempty" json:"value,omitempty"`
}
