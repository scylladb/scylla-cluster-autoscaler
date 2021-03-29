package metrics

import (
	"context"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"testing"
	"time"
)

func TestPrometheusProviderQuery(t *testing.T) {

	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})

	tests := []struct {
		name           string
		queryFun       func(string, time.Time) (model.Value, v1.Warnings, error)
		queryExpr      string
		expectedResult bool
		errorExpected  bool
	}{
		{
			name:           "Simple query function returns positively",
			queryFun:       mock.SimpleQueryFunction(),
			expectedResult: true,
		},
		{
			name:           "Query returns negatively",
			queryFun:       mock.SimpleQueryFunction(),
			queryExpr:      mock.QueryWillReturnFalse,
			expectedResult: false,
		},
		{
			name:          "Incorrect query expression",
			queryFun:      mock.SimpleQueryFunction(),
			queryExpr:     mock.IncorrectQueryExpr,
			errorExpected: true,
		},
		{
			name: "Query returns arbitrary error",
			queryFun: func(string, time.Time) (model.Value, v1.Warnings, error) {
				return nil, v1.Warnings{}, errors.New("Arbitrary error")
			},
			errorExpected: true,
		},
		{
			name: "Query returns unexpected value type",
			queryFun: func(string, time.Time) (model.Value, v1.Warnings, error) {
				return model.Matrix{}, v1.Warnings{}, nil
			},
			errorExpected: true,
		},
		{
			name: "Query returns empty result vector",
			queryFun: func(string, time.Time) (model.Value, v1.Warnings, error) {
				return model.Vector{}, v1.Warnings{}, nil
			},
			errorExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := mock.NewMockApi(test.queryFun, nil)
			p := NewPrometheusProvider(m, logger, time.Minute)
			if test.queryExpr == "" {
				test.queryExpr = mock.QueryWillReturnTrue
			}
			res, err := p.Query(ctx, test.queryExpr)
			if !test.errorExpected {
				if err != nil {
					t.Errorf("test \"%s\" error, err %v", test.name, err)
				} else if test.expectedResult != res {
					t.Errorf("test \"%s\" expected result %v, got %v", test.name, test.expectedResult, res)
				}
			}
		})
	}
}

func TestPrometheusProviderRangedQuery(t *testing.T) {

	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})

	tests := []struct {
		name           string
		rangedQueryFun func(string, v1.Range) (model.Value, v1.Warnings, error)
		queryExpr      string
		duration       time.Duration
		argStep        *time.Duration
		expectedResult bool
		errorExpected  bool
	}{
		{
			name:           "Simple ranged query returns positively",
			rangedQueryFun: mock.SimpleRangedQueryFunction(),
			expectedResult: true,
		},
		{
			name:           "Ranged query returns negatively",
			rangedQueryFun: mock.SimpleRangedQueryFunction(),
			queryExpr:      mock.QueryWillReturnFalse,
			expectedResult: false,
		},
		{
			name:           "Incorrect query expression",
			rangedQueryFun: mock.SimpleRangedQueryFunction(),
			queryExpr:      mock.IncorrectQueryExpr,
			errorExpected:  true,
		},
		{
			name: "Ranged query returns arbitrary error",
			rangedQueryFun: func(string, v1.Range) (model.Value, v1.Warnings, error) {
				return nil, v1.Warnings{}, errors.New("Arbitrary error")
			},
			errorExpected: true,
		},
		{
			name: "Ranged query returns unexpected value type",
			rangedQueryFun: func(string, v1.Range) (model.Value, v1.Warnings, error) {
				return model.Vector{}, v1.Warnings{}, nil
			},
			errorExpected: true,
		},
		{
			name: "Query returns empty result matrix",
			rangedQueryFun: func(string, v1.Range) (model.Value, v1.Warnings, error) {
				return model.Matrix{}, v1.Warnings{}, nil
			},
			errorExpected: true,
		},
		{
			name: "Query returns matrix containing empty record",
			rangedQueryFun: func(string, v1.Range) (model.Value, v1.Warnings, error) {
				return model.Matrix{{}}, v1.Warnings{}, nil
			},
			errorExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := mock.NewMockApi(nil, test.rangedQueryFun)
			p := NewPrometheusProvider(m, logger, time.Minute)
			if test.queryExpr == "" {
				test.queryExpr = mock.QueryWillReturnTrue
			}
			res, err := p.RangedQuery(ctx, test.queryExpr, test.duration, test.argStep)

			if !test.errorExpected {
				if err != nil {
					t.Errorf("test \"%s\" error, err %v", test.name, err)
				} else if test.expectedResult != res {
					t.Errorf("test \"%s\" expected result %v, got %v", test.name, test.expectedResult, res)
				}
			}
		})
	}
}
