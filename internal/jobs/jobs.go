package jobs

import (
	"github.com/go-co-op/gocron/v2"
	"github.com/panxiao81/e5renew/internal/db"
)

type JobScheduler struct {
	gocron.Scheduler
	db db.APILogStore
}

func NewJobScheduler(db db.APILogStore) (*JobScheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &JobScheduler{
		Scheduler: s,
		db:        db,
	}, nil
}
