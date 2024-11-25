package notifier

import (
	"context"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
	"github.com/kirychukyurii/notificator/config/notifiers"
	"github.com/kirychukyurii/notificator/model"
	"github.com/kirychukyurii/notificator/notifier/stdout"
	"github.com/kirychukyurii/notificator/notifier/webitel"
)

// Notifier notifies about alerts under constraints of the given context. It
// returns an error if unsuccessful and a flag whether the error is
// recoverable. This information is useful for a retry logic.
type Notifier interface {
	Notify(context.Context, *config.Technical, ...*model.Alert) (bool, error)
	String() string
}

func NewNotifiers(log *wlog.Logger, nrs *notifiers.Notifiers) ([]Notifier, error) {
	var (
		notifiers []Notifier
		add       = func(name string, account any, f func(name string, l *wlog.Logger) (Notifier, error)) {
			n, err := f(name, log.With(wlog.String("notifier", name), wlog.Any("account", account)))
			if err != nil {
				log.Error("skip notifier", wlog.Err(err))

				return
			}

			log.Info("add notifier", wlog.String("name", name), wlog.Any("account", account))

			notifiers = append(notifiers, n)
		}
	)

	if nrs.StdOut {
		add("stdout", "log", func(name string, l *wlog.Logger) (Notifier, error) { return stdout.New(name, l) })
	}

	// for _, c := range nrs.WebhookConfigs {
	// 	add("webhook", c.URL, func(l *wlog.Logger) (Notifier, error) { return telegram.New(c, l) })
	// }

	for _, c := range nrs.WebitelConfigs {
		add("webitel", c.URL, func(name string, l *wlog.Logger) (Notifier, error) { return webitel.New(name, c, l) })
	}

	return notifiers, nil
}
