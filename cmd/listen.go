package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/spf13/cobra"
	"github.com/webitel/wlog"
	"golang.org/x/sync/errgroup"

	"github.com/kirychukyurii/notificator/config"
	"github.com/kirychukyurii/notificator/listener"
)

func listenCommand(cfg *config.Config, log *wlog.Logger) *cobra.Command {
	c := &cobra.Command{
		Use:          "listen",
		Short:        "Listen incoming messages",
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetConsoleLevel(cfg.Logger.Level)
			app, err := New(cfg, log)
			if err != nil {
				return fmt.Errorf("app: %v", err)
			}

			// os.Interrupt to gracefully shutdown on Ctrl+C which is SIGINT
			// syscall.SIGTERM is the usual signal for termination and the default one (it can be modified)
			// for docker containers, which is also used by kubernetes.
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			// This blocks until the context is finished or until an error is produced
			if err = app.Run(ctx); err != nil {
				app.log.Error("run app", wlog.Err(err))
			}

			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()

			done := make(chan struct{}, 1)
			go func() {
				app.Cleanup(cleanupCtx)
				close(done)
			}()

			select {
			case <-done:
			case <-cleanupCtx.Done():
				app.log.Error("app failed to clean up in time")
			}

			return err
		},
	}

	return c
}

type App struct {
	cfg       *config.Config
	log       *wlog.Logger
	scheduler *listener.Scheduler

	// Closed once the App has finished starting
	startedCh chan struct{}
	errCh     chan error

	eg *errgroup.Group
}

func New(cfg *config.Config, log *wlog.Logger) (*App, error) {
	return &App{
		cfg:       cfg,
		log:       log,
		startedCh: make(chan struct{}),
		eg:        &errgroup.Group{},
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	// Notify anyone who might be listening that the App has finished starting.
	// This can be used by, e.g., app tests.
	defer close(a.startedCh)
	a.errCh = make(chan error, 100)

	timezone, err := time.LoadLocation(a.cfg.Timezone)
	if err != nil {
		return fmt.Errorf("load default timezone: %v", err)
	}

	scheduler := listener.NewScheduler(timezone)
	ls, err := listener.NewListeners(a.log, a.cfg.Listeners)
	if err != nil {
		return err
	}

	for i, start := range a.cfg.Listeners.Start {
		_, err := scheduler.ScheduleJob(start, fmt.Sprintf("start-%d", i), func(job gocron.Job) error {
			for _, l := range ls {
				if err := l.Listen(ctx); err != nil {
					return err
				}
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	for i, stop := range a.cfg.Listeners.Stop {
		_, err := scheduler.ScheduleJob(stop, fmt.Sprintf("stop-%d", i), func(job gocron.Job) error {
			for _, l := range ls {
				if err := l.Close(); err != nil {
					return err
				}
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	a.log.Info("app started")

	// App blocks until it receives a signal to exit
	// this signal may come from the node or from sig-abort (ctrl-c)
	select {
	case <-ctx.Done():
		return nil
	case err := <-a.errCh:
		return err
	}
}

func (a *App) Started() <-chan struct{} {
	return a.startedCh
}

// Cleanup stops all App services.
func (a *App) Cleanup(ctx context.Context) {
	a.log.Debug("app cleanup starting...")
	if err := a.eg.Wait(); err != nil {
		a.log.Error("cleanup resources", wlog.Err(err))
	}

	a.log.Info("app cleanup completed")
}
