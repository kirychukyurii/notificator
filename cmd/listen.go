package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/spf13/cobra"
	"github.com/webitel/wlog"
	"golang.org/x/sync/errgroup"

	"github.com/kirychukyurii/notificator/config"
	"github.com/kirychukyurii/notificator/listener"
	"github.com/kirychukyurii/notificator/manager"
	"github.com/kirychukyurii/notificator/notifier"
	"github.com/kirychukyurii/notificator/server"
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
			if err := cfg.Load(configPath); err != nil {
				return err
			}

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
	cfg *config.Config
	log *wlog.Logger

	scheduler *listener.Scheduler
	queue     *notifier.Queue
	srv       *server.Server

	mgr       *manager.Bot
	listeners []listener.Listener

	// Closed once the App has finished starting
	startedCh            chan struct{}
	initializedListeners chan struct{}
	errCh                chan error

	eg *errgroup.Group
}

func New(cfg *config.Config, log *wlog.Logger) (*App, error) {
	timezone, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load default timezone: %v", err)
	}

	scheduler := listener.NewScheduler(log, timezone)
	notifiers, err := notifier.NewNotifiers(log, cfg.Notifiers)
	if err != nil {
		return nil, err
	}

	mgr, err := manager.NewBot(cfg.Manager, log)
	if err != nil {
		return nil, err
	}

	q := notifier.NewQueue(log, cfg.GroupWait, notifiers, mgr)
	srv, err := server.New(log, cfg.HttpServer)
	if err != nil {
		return nil, err
	}

	app := &App{
		cfg:                  cfg,
		log:                  log,
		scheduler:            scheduler,
		queue:                q,
		srv:                  srv,
		mgr:                  mgr,
		startedCh:            make(chan struct{}),
		eg:                   &errgroup.Group{},
		initializedListeners: make(chan struct{}),
	}

	go func() {
		app.listeners = listener.NewListeners(log, cfg, q, srv)
		app.initializedListeners <- struct{}{}
	}()

	return app, nil
}

// TODO: Panic recovery
// 	1. App down
// 	2. App start
// 	3. Check if time between start/stop intervals
// 	4. If yes - read onduty from file, skip next run
// 	5. Start listeners
// 	6. If no - start as regular

func (a *App) Run(ctx context.Context) error {
	// Notify anyone who might be listening that the App has finished starting.
	// This can be used by, e.g., app tests.
	defer close(a.startedCh)
	a.errCh = make(chan error, 100)
	go func() {
		if err := a.srv.Start(); err != nil {
			a.errCh <- err
		}
	}()

	go a.queue.Process(ctx)

	// FIXME: Wait until all listeners are initialized blocked app and dont allow to exit
	<-a.initializedListeners

	logSchedJob := func(job gocron.Job) {
		a.log.Info("start scheduled job", wlog.Any("tags", job.Tags()), wlog.Any("next_run_at", job.NextRun()), wlog.Int("run_count", job.RunCount()))
	}

	for i, start := range a.cfg.Start {
		f := func(job gocron.Job) error {
			logSchedJob(job)
			if err := a.mgr.ChooseTechnicals(a.cfg.Technicals); err != nil {
				return err
			}

			var onduty *config.Technical
			select {
			case phone := <-a.mgr.OnDuty():
				for _, t := range a.cfg.Technicals {
					if t.Phone == phone {
						t.OnDuty = true
						onduty = t
					}
				}
			}

			// if err := a.mgr.Close(); err != nil {
			// 	return err
			// }

			a.queue.WithOnDuty(onduty)
			wg := &sync.WaitGroup{}
			for _, l := range a.listeners {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := l.Listen(ctx); err != nil {
						a.log.Error("listen events", wlog.Err(err), wlog.String("listener", l.String()))
					}
				}()
			}

			wg.Wait()

			return nil
		}

		_, err := a.scheduler.ScheduleJob(start, fmt.Sprintf("start-%d", i), f)
		if err != nil {
			return err
		}
	}

	for i, stop := range a.cfg.Stop {
		f := func(job gocron.Job) error {
			logSchedJob(job)
			for _, l := range a.listeners {
				if err := l.Close(); err != nil {
					return err
				}
			}

			return nil
		}

		_, err := a.scheduler.ScheduleJob(stop, fmt.Sprintf("stop-%d", i), f)
		if err != nil {
			return err
		}
	}

	a.log.Info("app started, wait for scheduled jobs")

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

	for _, l := range a.listeners {
		a.eg.Go(func() error {
			if err := l.Close(); err != nil {
				a.log.Info("close listener", wlog.Err(err), wlog.String("listener", l.String()))
			}

			return nil
		})
	}

	if err := a.eg.Wait(); err != nil {
		a.log.Error("cleanup resources", wlog.Err(err))
	}

	a.log.Info("app cleanup completed")
}
