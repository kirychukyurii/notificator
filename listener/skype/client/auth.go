package client

import (
	"context"
	"strconv"
	"time"

	"github.com/webitel/wlog"
)

const ApiMsacc string = "https://login.live.com"

func (c *Client) Login() error {
	c.provider = newAuthenticationProvider(c.httpcli, c.username)
	token, expires, err := c.provider.Auth(c.password)
	if err != nil {
		return err
	}

	c.skypeToken = token
	tokenExpires, err := strconv.Atoi(expires)
	if err != nil {
		return err
	}

	c.skypeTokenExpires = tokenExpires

	return nil
}

func (c *Client) skypeTokenWatcher(ctx context.Context) error {
	errCh := make(chan error, 1)
	ticker := time.NewTicker(time.Duration(c.skypeTokenExpires-100) * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				c.log.Debug("refresh skype, registration token and resubscribe to events")
				if err := c.Login(); err != nil {
					errCh <- err
				}

				c.endpoint.skypeToken = c.skypeToken
				if err := c.endpoint.registrationToken(); err != nil {
					errCh <- err
				}

				if err := c.endpoint.Subscribe(); err != nil {
					errCh <- err
				}
			}
		}
	}()

	c.log.Debug("skype token watcher started", wlog.Any("interval", time.Duration(c.skypeTokenExpires-100)*time.Second))

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
