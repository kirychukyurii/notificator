package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/telegram/updates/hook"
	"github.com/gotd/td/tg"
	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config/listeners"
	"github.com/kirychukyurii/notificator/model"
	"github.com/kirychukyurii/notificator/notifier"
)

type Telegram struct {
	log   *wlog.Logger
	cfg   *listeners.TelegramConfig
	queue *notifier.Queue
	cli   *telegram.Client
	gaps  *updates.Manager

	listen *atomic.Bool

	stopFunc stopFunc
}

func New(cfg *listeners.TelegramConfig, sessionDir string, log *wlog.Logger, queue *notifier.Queue) (*Telegram, error) {
	// Setting up session storage.
	// This is needed to reuse session and not login every time.
	dir := filepath.Join(sessionDir, sessionFolder(cfg.Phone))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	// So, we are storing session information in current directory, under subdirectory "session/phone_hash"
	sessionStorage := &telegram.FileSessionStorage{
		Path: filepath.Join(dir, "session.json"),
	}

	// Dispatcher is used to register handlers for events.
	dispatcher := tg.NewUpdateDispatcher()

	listen := &atomic.Bool{}

	dispatcher.OnNewMessage(onNewMessage(listen, queue))
	dispatcher.OnNewChannelMessage(onNewChannelMessage(listen, queue))
	gaps := updates.New(updates.Config{
		Handler: dispatcher,
	})

	options := telegram.Options{
		SessionStorage: sessionStorage, // Setting up session sessionStorage to store auth data.
		UpdateHandler:  gaps,
		Middlewares: []telegram.Middleware{
			hook.UpdateHook(gaps.Handle),
		},
	}

	client := telegram.NewClient(cfg.AppID, cfg.AppHash, options)
	stop, err := connect(context.TODO(), client)
	if err != nil {
		return nil, err
	}

	// Authentication flow handles authentication process, like prompting for code and 2FA password.
	flow := auth.NewFlow(Auth{phone: cfg.Phone}, auth.SendCodeOptions{})
	if err := client.Auth().IfNecessary(context.Background(), flow); err != nil {
		return nil, err
	}

	self, err := client.Self(context.TODO())
	if err != nil {
		return nil, err
	}

	log.Info("logged user", wlog.String("first_name", self.FirstName), wlog.String("last_name", self.LastName),
		wlog.String("username", self.Username), wlog.Int64("id", self.ID))

	return &Telegram{
		log:      log,
		cfg:      cfg,
		queue:    queue,
		cli:      client,
		gaps:     gaps,
		listen:   listen,
		stopFunc: stop,
	}, nil
}

func (t *Telegram) Listen(ctx context.Context) error {
	status, err := t.cli.Auth().Status(ctx)
	if err != nil {
		return err
	}

	if !status.Authorized {
		return fmt.Errorf("telegram: not authorized")
	}

	opts := updates.AuthOptions{
		IsBot: status.User.Bot,
		OnStart: func(ctx context.Context) {
			t.log.Info("update recovery initialized and started, listening for events")
		},
	}

	t.listen.Store(true)
	defer t.listen.Store(false)
	if err := t.gaps.Run(ctx, t.cli.API(), status.User.ID, opts); err != nil {
		return fmt.Errorf("update recovery initialization: %v", err)
	}

	return nil
}

func (t *Telegram) String() string {
	return "telegram"
}

func (t *Telegram) Close() error {
	if t.stopFunc != nil {
		return t.stopFunc()
	}

	return nil
}

// stopFunc closes Client and waits until Run returns.
type stopFunc func() error

// Connect blocks until a client is connected,
// calling Run internally in the background.
func connect(ctx context.Context, client *telegram.Client) (stopFunc, error) {
	ctx, cancel := context.WithCancel(ctx)
	errC := make(chan error, 1)
	initDone := make(chan struct{})
	go func() {
		defer close(errC)
		errC <- client.Run(ctx, func(ctx context.Context) error {
			close(initDone)
			<-ctx.Done()
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}

			return ctx.Err()
		})
	}()

	select {
	case <-ctx.Done(): // context canceled
		cancel()

		return func() error { return nil }, ctx.Err()
	case err := <-errC: // startup timeout
		cancel()

		return func() error { return nil }, err
	case <-initDone: // init done
	}

	stopFn := func() error {
		cancel()

		return <-errC
	}

	return stopFn, nil
}

func sessionFolder(phone string) string {
	var out []rune
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			out = append(out, r)
		}
	}

	return "phone-" + string(out)
}

// onNewMessage handles new private messages or messages in a basic group.
// See: https://core.telegram.org/constructor/updateNewMessage
func onNewMessage(listen *atomic.Bool, queue *notifier.Queue) tg.NewMessageHandler {
	return func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		if !listen.Load() {
			return nil
		}

		msg, ok := update.Message.(*tg.Message)
		if !ok {
			return nil
		}

		if msg.Out {
			// Outgoing message.
			return nil
		}

		if _, ok := msg.GetViaBotID(); ok {
			return nil
		}

		queue.Push(&notifier.Message{
			Channel: "telegram",
			Content: &model.Alert{
				Channel: "telegram",
				Text:    msg.Message,
			},
		})

		return nil
	}
}

// onNewMessage handles new messages in channel/supergroup.
// See: https://core.telegram.org/constructor/updateNewChannelMessage
func onNewChannelMessage(listen *atomic.Bool, queue *notifier.Queue) tg.NewChannelMessageHandler {
	return func(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
		if !listen.Load() {
			return nil
		}

		msg, ok := update.Message.(*tg.Message)
		if !ok {
			return nil
		}

		if msg.Out {
			// Outgoing message.
			return nil
		}

		if _, ok := msg.GetViaBotID(); ok {
			return nil
		}

		queue.Push(&notifier.Message{
			Channel: "telegram",
			Content: &model.Alert{
				Channel: "telegram",
				Text:    msg.Message,
			},
		})

		return nil
	}
}
