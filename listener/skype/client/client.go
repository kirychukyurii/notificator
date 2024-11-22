package client

import (
	"context"

	"github.com/webitel/wlog"
)

type Client struct {
	log     *wlog.Logger
	httpcli *httpClient

	provider authenticationProvider

	username string
	password string

	skypeToken        string
	skypeTokenExpires int

	endpoint *endpoint
	handlers []Handler
}

func New(log *wlog.Logger, username, password string) (*Client, error) {
	client := &Client{
		log:      log,
		httpcli:  newHttpClient(log),
		username: username,
		password: password,
		handlers: make([]Handler, 0),
	}

	if err := client.Login(); err != nil {
		return nil, err
	}

	go client.skypeTokenWatcher(context.TODO())

	e, err := newEndpoint(log, client.httpcli, client.skypeToken)
	if err != nil {
		return nil, err
	}

	client.endpoint = e

	return client, nil
}

// Poll create event loop, that used to retrieve a list of events since the last poll.
func (c *Client) Poll(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		for {
			events, err := c.endpoint.Events()
			if err != nil {
				errCh <- err
			}

			for _, e := range events {
				c.handle(e)
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (c *Client) Stop(ctx context.Context) error {
	return c.endpoint.Unsubscribe()
}
