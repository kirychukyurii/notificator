package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	boltstor "github.com/gotd/contrib/bbolt"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/contrib/storage"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/telegram/query"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/webitel/wlog"
	"go.etcd.io/bbolt"
	"golang.org/x/time/rate"

	"github.com/kirychukyurii/notificator/config"
)

type Telegram struct {
	log *wlog.Logger
	cfg *config.TelegramConfig

	tg *tgres

	cancel context.CancelFunc
}

type tgres struct {
	auth            auth.Flow
	peerDB          *boltstor.PeerStorage
	waiter          *floodwait.Waiter
	updatesRecovery *updates.Manager
	cli             *telegram.Client
}

func New(cfg *config.TelegramConfig, log *wlog.Logger) (*Telegram, error) {
	t := &Telegram{
		log: log,
		cfg: cfg,
	}

	// Setting up session storage.
	// This is needed to reuse session and not login every time.
	sessionDir := filepath.Join("session", sessionFolder(cfg.Phone))
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return nil, err
	}

	// So, we are storing session information in current directory, under subdirectory "session/phone_hash"
	sessionStorage := &telegram.FileSessionStorage{
		Path: filepath.Join(sessionDir, "session.json"),
	}

	// Setting up client.
	//
	// Dispatcher is used to register handlers for events.
	dispatcher := tg.NewUpdateDispatcher()

	// Registering handler for new private messages.
	dispatcher.OnNewMessage(t.onNewMessage)

	// Setting up persistent storage for qts/pts to be able to
	// recover after restart.
	boltdb, err := bbolt.Open(filepath.Join(sessionDir, "updates.bolt.db"), 0666, nil)
	if err != nil {
		return nil, fmt.Errorf("create bolt storage: %v", err)
	}

	peerDB := boltstor.NewPeerStorage(boltdb, []byte("peer_store"))

	// Setting up update handler that will fill peer storage before
	// calling dispatcher handlers.
	updateHandler := storage.UpdateHook(dispatcher, peerDB)
	updatesRecovery := updates.New(updates.Config{
		Handler: updateHandler, // using previous handler with peerDB
		Storage: boltstor.NewStateStorage(boltdb),
	})

	// Handler of FLOOD_WAIT that will automatically retry request.
	waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
		log.Warn("got FLOOD_WAIT, retry after", wlog.Any("retry", wait.Duration))
	})

	options := telegram.Options{
		SessionStorage: sessionStorage,  // Setting up session sessionStorage to store auth data.
		UpdateHandler:  updatesRecovery, // Setting up handler for updates from server.
		Middlewares: []telegram.Middleware{

			// Setting up FLOOD_WAIT handler to automatically wait and retry request.
			waiter,

			// Setting up general rate limits to less likely get flood wait errors.
			ratelimit.New(rate.Every(time.Millisecond*100), 5),
		},
	}

	client := telegram.NewClient(cfg.AppID, cfg.AppHash, options)

	// Setting up resolver cache that will use peer storage.
	resolver := storage.NewResolverCache(peer.Plain(client.API()), peerDB)
	// Usage:
	//   if _, err := resolver.ResolveDomain(ctx, "tdlibchat"); err != nil {
	//	   return errors.Wrap(err, "resolve")
	//   }
	_ = resolver

	// Authentication flow handles authentication process, like prompting for code and 2FA password.
	flow := auth.NewFlow(Auth{phone: cfg.Phone}, auth.SendCodeOptions{})

	res := &tgres{
		auth:            flow,
		peerDB:          peerDB,
		waiter:          waiter,
		updatesRecovery: updatesRecovery,
		cli:             client,
	}

	t.tg = res

	return t, nil
}

func (t *Telegram) Listen(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	t.cancel = cancel
	exit := make(chan error, 1)

	go func() {
		defer close(exit)
		exit <- t.tg.waiter.Run(ctx, func(ctx context.Context) error {
			if err := t.tg.cli.Run(ctx, func(ctx context.Context) error {
				// Perform auth if no session is available.
				if err := t.tg.cli.Auth().IfNecessary(ctx, t.tg.auth); err != nil {
					return fmt.Errorf("auth: %v", err)
				}

				// Getting info about current user.
				self, err := t.tg.cli.Self(ctx)
				if err != nil {
					return fmt.Errorf("call self: %v", err)
				}

				t.log.Info("logged user", wlog.String("first_name", self.FirstName), wlog.String("last_name", self.LastName),
					wlog.String("username", self.Username), wlog.Int64("id", self.ID))

				if t.cfg.FillPeersOnStart {
					t.log.Info("filling peer storage from dialogs to cache entities")
					collector := storage.CollectPeers(t.tg.peerDB)
					if err := collector.Dialogs(ctx, query.GetDialogs(t.tg.cli.API()).Iter()); err != nil {
						return fmt.Errorf("collect peers: %v", err)
					}
				}

				t.log.Info("listening for updates")
				return t.tg.updatesRecovery.Run(ctx, t.tg.cli.API(), self.ID, updates.AuthOptions{
					IsBot: self.Bot,
					OnStart: func(ctx context.Context) {
						t.log.Info("update recovery initialized and started, listening for events")
					},
				})
			}); err != nil {
				return err
			}

			return nil
		})
	}()

	select {
	case <-ctx.Done(): // context canceled
		return ctx.Err()
	case err := <-exit: // startup timeout
		return err
	}
}

func (t *Telegram) onNewMessage(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
	msg, ok := u.Message.(*tg.Message)
	if !ok {
		return nil
	}
	if msg.Out {
		// Outgoing message.
		return nil
	}

	// Use PeerID to find peer because *Short updates does not contain any entities, so it necessary to
	// store some entities.
	//
	// Storage can be filled using PeerCollector (i.e. fetching all dialogs first).
	p, err := storage.FindPeer(ctx, t.tg.peerDB, msg.GetPeerID())
	if err != nil {
		return err
	}

	t.log.Info("recv message", wlog.String("peer", p.String()))

	return nil
}

func (t *Telegram) String() string {
	return "telegram"
}

func (t *Telegram) Close() error {
	if t.cancel != nil {
		t.cancel()
	}

	return nil
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
