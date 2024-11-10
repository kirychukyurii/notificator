package manager

import (
	"context"
	"fmt"

	"github.com/fasthttp/router"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/valyala/fasthttp"
	"golang.ngrok.com/ngrok"
	nc "golang.ngrok.com/ngrok/config"
	"golang.org/x/sync/errgroup"

	"github.com/kirychukyurii/notificator/config"
)

type Bot struct {
	cli *telego.Bot

	message *telego.SendMessageParams
	updates <-chan telego.Update

	eg *errgroup.Group
}

func NewBot(cfg *config.Manager) (*Bot, error) {
	bot, err := telego.NewBot(cfg.BotID)
	if err != nil {
		return nil, err
	}

	// Create a new Ngrok tunnel to connect local network with the Internet & have HTTPS domain for bot
	tun, err := ngrok.Listen(context.Background(),
		// Forward connections to localhost:8080
		nc.HTTPEndpoint(nc.WithForwardsTo(":8080")),
		// Authenticate into Ngrok using NGROK_AUTHTOKEN env (optional)
		ngrok.WithAuthtokenFromEnv(),
	)

	// Prepare fast HTTP server
	srv := &fasthttp.Server{}

	// Get an update channel from webhook using Ngrok
	updates, _ := bot.UpdatesViaWebhook("/bot"+bot.Token(),
		// Set func server with fast http server inside that will be used to handle webhooks
		telego.WithWebhookServer(telego.FuncWebhookServer{
			Server: telego.FastHTTPWebhookServer{
				Logger: bot.Logger(),
				Server: srv,
				Router: router.New(),
			},
			// Override default start func to use Ngrok tunnel
			// Note: When server is stopped, the Ngrok tunnel always returns an error, so it should be handled by user
			StartFunc: func(_ string) error {
				return srv.Serve(tun)
			},
		}),

		// Calls SetWebhook before starting webhook and provide dynamic Ngrok tunnel URL
		telego.WithWebhookSet(&telego.SetWebhookParams{
			URL: tun.URL() + "/bot" + bot.Token(),
		}),
	)

	return &Bot{
		cli:     bot,
		message: tu.Message(tu.ID(cfg.ChatID), "Choose technical"),
		updates: updates,
		eg:      &errgroup.Group{},
	}, nil
}

func (b *Bot) Listen() error {
	b.eg.Go(func() error {
		return b.cli.StartWebhook("")
	})

	b.eg.Go(func() error {
		for update := range b.updates {
			fmt.Printf("Update: %+v\n", update)
		}

		return nil
	})

	if err := b.eg.Wait(); err != nil {
		return err
	}

	return nil
}

func (b *Bot) Close() error {
	return b.cli.Close()
}
