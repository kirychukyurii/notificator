package teams

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config/listeners"
	"github.com/kirychukyurii/notificator/model"
	"github.com/kirychukyurii/notificator/notifier"
	"github.com/kirychukyurii/notificator/server"
)

const (
	initialBackoff = 5 * time.Second
	maxBackoff     = 1 * time.Minute
	maxRetries     = 5
	backoffFactor  = 2.0
)

var (
	scopes = []string{
		"https://graph.microsoft.com/.default",
	}
)

type auth struct {
	log       *wlog.Logger
	cfg       *listeners.TeamsConfig
	queue     *notifier.Queue
	publicURL string

	cli *confidential.Client

	code  chan string
	token *confidential.AuthResult
	errCh chan error
}

func newAuth(ctx context.Context, cfg *listeners.TeamsConfig, log *wlog.Logger, srv *server.Server, queue *notifier.Queue) (*auth, error) {
	cred, err := confidential.NewCredFromSecret(cfg.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("create confidential credential: %w", err)
	}

	app, err := confidential.New("https://login.microsoftonline.com/"+cfg.TenantID, cfg.ClientID, cred)
	if err != nil {
		return nil, fmt.Errorf("create confidential client: %w", err)
	}

	a := &auth{
		log:       log,
		cfg:       cfg,
		queue:     queue,
		publicURL: srv.PublicURL(),
		cli:       &app,
		code:      make(chan string),
	}

	srv.HandleFunc("/auth/callback", a.handleAuthCallback)

	// Initial token acquisition with retries
	if err = a.acquireTokenWithRetry(ctx); err != nil {
		return nil, fmt.Errorf("initial token acquisition: %w", err)
	}

	go a.tokenRefreshLoop(ctx)

	return a, nil
}

func (a *auth) acquireToken(ctx context.Context) error {
	token, err := a.cli.AcquireTokenSilent(ctx, scopes)
	if err != nil {
		opts := []confidential.AuthCodeURLOption{
			confidential.WithTenantID(a.cfg.TenantID),
			confidential.WithLoginHint(a.cfg.Login),
		}

		redirectURL := a.publicURL + "/auth/callback"
		url, err := a.cli.AuthCodeURL(ctx, a.cfg.ClientID, redirectURL, scopes, opts...)
		if err != nil {
			return fmt.Errorf("get auth URL: %w", err)
		}

		a.queue.Push(&notifier.Message{
			Channel: "auth_code_url",
			Content: &model.AuthCodeURL{
				URL: url,
			},
		})

		authCode := <-a.code
		if authCode == "" {
			return fmt.Errorf("received empty auth code")
		}

		token, err = a.cli.AcquireTokenByAuthCode(ctx, authCode, redirectURL, scopes, confidential.WithTenantID(a.cfg.TenantID))
		if err != nil {
			return fmt.Errorf("acquire token by username and password: %w", err)
		}

		a.queue.Push(&notifier.Message{
			Channel: "resolve_auth_code_url",
			Content: &model.AuthCodeURL{
				URL: url,
			},
		})
	}

	a.token = &token

	return nil
}

// acquireTokenWithRetry attempts to acquire a token with exponential backoff and max retries.
func (a *auth) acquireTokenWithRetry(ctx context.Context) error {
	var err error
	backoff := initialBackoff

	for attempt := 0; attempt < maxRetries; attempt++ {
		err = a.acquireToken(ctx)
		if err == nil {
			return nil // Success.
		}

		if attempt == maxRetries-1 {
			break // Last attempt failed.
		}

		a.log.Warn("token acquisition attempt failed", wlog.Err(err), wlog.Int("attempt", attempt+1), wlog.String("retry_in", backoff.String()))

		// Add jitter to avoid thundering herd problem (±20% randomness).
		jitter := time.Duration(float64(backoff) * (0.8 + 0.4*rand.Float64()))
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during token acquisition retry: %w", ctx.Err())

		case <-time.After(jitter): // Continue with next attempt.
		}

		// Increase backoff for next attempt.
		backoff = time.Duration(float64(backoff) * backoffFactor)
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return fmt.Errorf("reached max attempts (%d) to acquire token: %w", maxRetries, err)
}

// tokenRefreshLoop keeps the token refreshed before it expires.
func (a *auth) tokenRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(a.token.ExpiresOn.Sub(time.Now()))
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				a.log.Info("token refresh loop stopped due to context cancellation")

				return
			case <-ticker.C:
				a.log.Debug("token approaching expiry, initiating refresh")
				if err := a.acquireTokenWithRetry(ctx); err != nil {
					a.errCh <- err

					return
				}

				ticker.Reset(a.token.ExpiresOn.Sub(time.Now()))
			}
		}
	}()
}

func (a *auth) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	a.code <- code

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("received code"))
}
