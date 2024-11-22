package notifier

var DefaultWebitelConfig = WebitelConfig{
	Authorization: &Authorization{
		Header: "X-Webitel-Access",
	},
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
