package jobs

import (
	"github.com/go-co-op/gocron/v2"
)

type JobScheduler struct {
	gocron.Scheduler
}

func NewJobScheduler() (*JobScheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &JobScheduler{
		Scheduler: s,
	}, nil
}
