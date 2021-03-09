package metrics

import (
	"context"
	"github.com/scylladb/go-log"
	"time"
)

type Provider interface {
	Query(ctx context.Context, expression string) (bool, error)

	RangedQuery(ctx context.Context, expression string, duration time.Duration, argStep *time.Duration) (bool, error)
}

type provider struct {
	logger      log.Logger
	defaultStep time.Duration
}
