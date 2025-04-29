package listeners

var DefaultTeamsConfig = TeamsConfig{}

type TeamsConfig struct {
	TenantID      string `yaml:"tenant_id" json:"tenant_id"`
	ClientID      string `yaml:"client_id" json:"client_id"`
	ClientSecret  string `yaml:"client_secret" json:"client_secret"`
	HomeAccountID string `yaml:"home_account_id" json:"home_account_id"`
	Login         string `yaml:"login" json:"login"`
	Password      string `yaml:"password" json:"password"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *TeamsConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultTeamsConfig
	type plain TeamsConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return nil
}
