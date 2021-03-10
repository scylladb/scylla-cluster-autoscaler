package metrics

import (
	"context"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/scylladb/go-log"
)

type Provider interface {
	FetchMetric(ctx context.Context, metric string, labels map[string]string) (float64, error)
}

type provider struct {
	api    v1.API
	logger log.Logger
}
