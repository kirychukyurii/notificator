package client

import (
	"fmt"
	"time"

	"github.com/webitel/wlog"
)

const ApiMsacc string = "https://login.live.com"

type auth struct {
	log      *wlog.Logger
	httpcli  *httpClient
	provider authenticationProvider

	username string
	token    string
	expires  time.Duration

	notify chan<- error
}

func newAuth(log *wlog.Logger, httpcli *httpClient, username, password string) (*auth, error) {
	a := &auth{
		log:      log,
		httpcli:  httpcli,
		provider: newAuthenticationProvider(httpcli, username),
		username: username,
		notify:   make(chan error),
	}

	if err := a.Login(password); err != nil {
		return nil, err
	}

	return a, nil
}

// Login performs authentication in two ways:
//   - Live - for accounts with Skype usernames and phone numbers (requires
//     calling out to the MS OAuth page, and retrieving the Skype token);
//   - SOAP - performs SOAP login (authentication with a Microsoft account
//     email address and password (or application-specific token), using
//     an endpoint to obtain a security token, and exchanging that for a Skype token).
func (a *auth) Login(password string) error {
	token, expires, err := a.provider.Auth(password)
	if err != nil {
		return err
	}

	a.token = token
	if a.expires, err = time.ParseDuration(expires + "s"); err != nil {
		return err
	}

	go func() {
		a.log.Info("start skype token expires watcher", wlog.Duration("expires", a.expires))

		ticker := time.NewTicker(a.expires)
		defer ticker.Stop()

		select {
		case <-ticker.C:
			a.notify <- fmt.Errorf("skype token has been expired")
		}
	}()

	return nil
}

func (a *auth) NotifyRefresh(notify chan<- error) {
	a.notify = notify
}
