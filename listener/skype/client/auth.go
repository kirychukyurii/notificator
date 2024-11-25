package client

import (
	"context"
	"strconv"
	"time"

	"github.com/webitel/wlog"
)

const ApiMsacc string = "https://login.live.com"

// Login performs authentication in two ways:
//   - Live - for accounts with Skype usernames and phone numbers (requires calling out to the MS OAuth page, and retrieving the Skype token);
//   - SOAP - performs SOAP login (authentication with a Microsoft account email address and password (or application-specific token), using an endpoint to obtain a security token, and exchanging that for a Skype token).
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
	ticker := time.NewTicker(time.Duration(c.skypeTokenExpires-200) * time.Second)
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
