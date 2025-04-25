package listener

import (
	"context"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
	"github.com/kirychukyurii/notificator/listener/skype"
	"github.com/kirychukyurii/notificator/listener/teams"
	"github.com/kirychukyurii/notificator/listener/telegram"
	"github.com/kirychukyurii/notificator/listener/webhook"
	"github.com/kirychukyurii/notificator/notifier"
	"github.com/kirychukyurii/notificator/server"
)

type Listener interface {
	Listen(ctx context.Context) error

	// String listener's code name
	String() string

	// Close shuts down listener and all it's running session(s)
	Close() error
}

func NewListeners(log *wlog.Logger, cfg *config.Config, queue *notifier.Queue, srv *server.Server) []Listener {
	var (
		listeners []Listener
		add       = func(name string, account any, f func(l *wlog.Logger) (Listener, error)) {
			n, err := f(log.With(wlog.String("listener", name), wlog.Any("account", account)))
			if err != nil {
				log.Error("skip listener", wlog.Err(err))

				return
			}

			log.Info("add listener", wlog.String("name", name), wlog.Any("account", account))
			listeners = append(listeners, n)
		}
	)

	for _, c := range cfg.Listeners.TelegramConfigs {
		add("telegram", c.Phone, func(l *wlog.Logger) (Listener, error) { return telegram.New(c, cfg.SessionsDir, l, queue) })
	}

	for _, c := range cfg.Listeners.SkypeConfigs {
		add("skype", c.Login, func(l *wlog.Logger) (Listener, error) { return skype.New(c, l, queue) })
	}

	if len(cfg.Listeners.WebhookConfigs) > 0 {
		handler := webhook.NewHandler(srv)
		for _, c := range cfg.Listeners.WebhookConfigs {
			add("webhook", c.Name, func(l *wlog.Logger) (Listener, error) { return webhook.New(c, l, queue, handler) })
		}
	}

	for _, c := range cfg.Listeners.TeamsConfigs {
		add("teams", c.Login, func(l *wlog.Logger) (Listener, error) { return teams.New(c, l, queue, srv) })
	}

	return listeners
}
