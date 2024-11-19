package listener

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/webitel/wlog"
)

type Scheduler struct {
	log  *wlog.Logger
	cron *gocron.Scheduler
}

func NewScheduler(log *wlog.Logger, location *time.Location) *Scheduler {
	scheduler := gocron.NewScheduler(location)
	scheduler.StartAsync()

	return &Scheduler{
		log:  log,
		cron: scheduler,
	}
}

// Stop the scheduler
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) ScheduleJob(interval string, jobTag string, jobFun interface{}, params ...interface{}) (*gocron.Job, error) {
	job, err := s.cron.Cron(interval).Tag(jobTag).DoWithJobDetails(jobFun, params...)
	if err != nil {
		return nil, fmt.Errorf("scheduling job: %v", err)
	}

	s.log.Info("schedule job", wlog.Any("tags", job.Tags()), wlog.String("interval", interval), wlog.Any("next_run_at", job.NextRun()))

	return job, nil
}
