package listeners

type Listeners struct {
	TelegramConfigs []*TelegramConfig `yaml:"telegram_configs" json:"telegram_configs"`
	SlackConfigs    []*SlackConfig    `yaml:"slack_configs" json:"slack_configs"`
	SkypeConfigs    []*SkypeConfig    `yaml:"skype_configs" json:"skype_configs"`
	WebhookConfigs  []*WebhookConfig  `yaml:"webhook_configs" json:"webhook_configs"`
}
