package application

import "time"

type Service struct {
	store Repository
	now   func() time.Time
}

func New(store Repository) *Service {
	return &Service{store: store, now: time.Now}
}
