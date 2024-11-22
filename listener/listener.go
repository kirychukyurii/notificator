package listener

import (
	"context"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config/listener"
	"github.com/kirychukyurii/notificator/listener/skype"
	"github.com/kirychukyurii/notificator/listener/telegram"
	"github.com/kirychukyurii/notificator/notifier"
)

type Listener interface {
	Listen(ctx context.Context) error

	// String listener's code name
	String() string

	// Close shuts down listener and all it's running session(s)
	Close() error
}

func NewListeners(log *wlog.Logger, lrs *listener.Listeners, queue *notifier.Queue) ([]Listener, error) {
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

	for _, c := range lrs.TelegramConfigs {
		add("telegram", c.Phone, func(l *wlog.Logger) (Listener, error) { return telegram.New(c, l, queue) })
	}

	// for _, c := range lrs.SlackConfigs {
	// 	add("slack", c.AppToken, func(l *wlog.Logger) (Listener, error) { return slack.New(c, l) })
	// }

	for _, c := range lrs.SkypeConfigs {
		add("skype", c.Login, func(l *wlog.Logger) (Listener, error) { return skype.New(c, l, queue) })
	}

	return listeners, nil
}
