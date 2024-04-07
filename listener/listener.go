package listener

import (
	"context"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
	"github.com/kirychukyurii/notificator/listener/telegram"
)

type Listener interface {
	Listen(ctx context.Context) error

	// String listener's code name
	String() string

	// Close shuts down listener and all it's running session(s)
	Close() error
}

func NewListeners(log *wlog.Logger, lrs *config.Listeners) ([]Listener, error) {
	var (
		listeners []Listener
		add       = func(name string, account any, lr any, f func(l *wlog.Logger) (Listener, error)) {
			n, err := f(log.With(wlog.String("listener", name), wlog.Any("account", account)))
			if err != nil {
				log.Error("skip listener", wlog.Err(err))

				return
			}

			listeners = append(listeners, n)
		}
	)

	for _, c := range lrs.TelegramConfigs {
		add("telegram", c.Phone, c, func(l *wlog.Logger) (Listener, error) { return telegram.New(c, l) })
	}

	return listeners, nil
}
