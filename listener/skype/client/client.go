package client

import (
	"context"
	"sync"
	"time"

	"github.com/webitel/wlog"
)

type Client struct {
	log     *wlog.Logger
	httpcli *httpClient

	profile        *auth
	endpoint       *endpoint
	mu             sync.Mutex
	connected      bool
	close          chan bool
	waitConnection chan struct{}

	handlers []Handler
}

func New(log *wlog.Logger, username, password string) (*Client, error) {
	client := &Client{
		log:            log,
		httpcli:        newHttpClient(log),
		close:          make(chan bool),
		waitConnection: make(chan struct{}),
		handlers:       make([]Handler, 0),
	}

	// Its bad case of nil == waitConnection, so close it at start.
	close(client.waitConnection)
	if err := client.connect(username, password); err != nil {
		return nil, err
	}

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

func (c *Client) checkConnect() error {
	c.mu.Lock()

	if c.connected {
		c.mu.Unlock()

		return nil
	}

	// Check it was closed.
	select {
	case <-c.close:
		c.close = make(chan bool)
	default:
		// Noop, new conn.
	}

	c.mu.Unlock()

	return nil
}

func (c *Client) tryConnect(username, password string) error {
	if err := c.checkConnect(); err != nil {
		return err
	}

	var err error
	if c.profile, err = newAuth(c.log, c.httpcli, username, password); err != nil {
		return err
	}

	if c.endpoint, err = newEndpoint(c.log, c.httpcli, c.profile.token); err != nil {
		return err
	}

	return nil
}

func (c *Client) reconnect(username, password string) {
	// Skip first connect.
	var connect bool

	for {
		if connect {
			if err := c.connect(username, password); err != nil {
				time.Sleep(1 * time.Second)

				continue
			}

			c.mu.Lock()
			c.connected = true
			c.mu.Unlock()

			// Unblock resubscribe a cycle.
			close(c.waitConnection)
		}

		connect = true
		skypeToken := make(chan error)
		c.profile.NotifyRefresh(skypeToken)

		endpointToken := make(chan error)
		c.endpoint.NotifyRefresh(endpointToken)

		// To avoid deadlocks, it is necessary to consume the messages from all channels.
		for skypeToken != nil || endpointToken != nil {
			select {
			case err := <-skypeToken:
				c.log.Error("skype token needs to refresh.. attempting to reconnect", wlog.Err(err))

				// Block all resubscribe attempt - they are useless because there is no connection
				c.mu.Lock()
				c.connected = false
				c.waitConnection = make(chan struct{})
				c.mu.Unlock()
				skypeToken = nil

			case err := <-endpointToken:
				c.log.Error("endpoint registration token needs to refresh.. attempting to reconnect", wlog.Err(err))

				// Block all resubscribe attempt - they are useless because there is no connection
				c.mu.Lock()
				c.connected = false
				c.waitConnection = make(chan struct{})
				c.mu.Unlock()
				endpointToken = nil

			case <-c.close:
				return
			}
		}
	}
}

func (c *Client) connect(username, password string) error {
	if err := c.tryConnect(username, password); err != nil {
		return err
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	// Create reconnect loop.
	go c.reconnect(username, password)

	return nil
}
