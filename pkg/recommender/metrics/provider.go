package metrics

import (
	"context"
	"github.com/scylladb/go-log"
	"time"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type Provider interface {
	Query(ctx context.Context, expression string) (bool, error)

	RangedQuery(ctx context.Context, expression string, duration time.Duration, argStep *time.Duration) (bool, error)

	SetApi(api v1.API)
}

type provider struct {
	logger      log.Logger
	defaultStep time.Duration
}
