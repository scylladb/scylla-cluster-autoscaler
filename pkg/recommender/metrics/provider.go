package metrics

import (
	"context"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/scylladb/go-log"
	"time"
)

type Provider interface {
	Query(ctx context.Context, expression string) (bool, error)

	RangedQuery(ctx context.Context, expression string, duration time.Duration) (bool, error)
}

type provider struct {
	api    v1.API
	logger log.Logger
}
