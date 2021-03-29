package metrics

import (
	"context"
	"github.com/scylladb/go-log"
	"time"
)

type Provider interface {
	// Perform an instant query to the metrics provider.
	// ctx - context.Context
	// expression - an expression to be queried
	// Return value: unless an error has occurred, return a boolean value corresponding to the evaluated expression.
	Query(ctx context.Context, expression string) (bool, error)

	// Perform a ranged query to the metrics provider.
	// ctx - context.Context
	// expression - an expression to be queried
	// duration - time range
	// step (optional) - minimal time range between each data point; optional, defaults to defaultStep
	// Return value: unless an error has occurred, return a boolean value corresponding to the evaluated expression,
	// i.e. whether the expression was true in each data point in the given time range.
	RangedQuery(ctx context.Context, expression string, duration time.Duration, argStep *time.Duration) (bool, error)
}

type provider struct {
	logger      log.Logger
	defaultStep time.Duration
}
