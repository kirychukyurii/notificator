package listener

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron"
)

type Scheduler struct {
	Cron *gocron.Scheduler
}

func NewScheduler(location *time.Location) *Scheduler {
	scheduler := gocron.NewScheduler(location)
	scheduler.StartAsync()

	return &Scheduler{
		Cron: scheduler,
	}
}

// Stop the scheduler
func (s *Scheduler) Stop() {
	s.Cron.Stop()
}

func (s *Scheduler) ScheduleJob(interval string, jobTag string, jobFun interface{}, params ...interface{}) (*gocron.Job, error) {
	job, err := s.Cron.Cron(interval).Tag(jobTag).DoWithJobDetails(jobFun, params...)
	if err != nil {
		return nil, fmt.Errorf("scheduling job: %v", err)
	}

	return job, nil
}
