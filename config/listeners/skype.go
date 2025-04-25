package listeners

var DefaultSkypeConfig = SkypeConfig{}

type SkypeConfig struct {
	Login    string `yaml:"login" json:"login"`
	Password string `yaml:"password" json:"password"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *SkypeConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultSkypeConfig
	type plain SkypeConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return nil
}
