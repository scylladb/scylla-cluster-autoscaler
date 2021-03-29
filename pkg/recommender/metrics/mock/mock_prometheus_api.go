package mock

import (
	"context"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"time"
)

type mockApi struct {
	q  func(query string, ts time.Time) (model.Value, v1.Warnings, error)
	qr func(query string, r v1.Range) (model.Value, v1.Warnings, error)
}

func NewMockApi(q func(query string, ts time.Time) (model.Value, v1.Warnings, error),
	qr func(query string, r v1.Range) (model.Value, v1.Warnings, error)) v1.API {

	defaultQ := func(query string, ts time.Time) (model.Value, v1.Warnings, error) {
		return nil, []string{}, errors.New("Query function in mock_prometheus_api not implemented")
	}
	defaultQr := func(query string, r v1.Range) (model.Value, v1.Warnings, error) {
		return nil, []string{}, errors.New("QueryRange function in mock_prometheus_api not implemented")
	}

	return &mockApi{
		q: func() func(query string, ts time.Time) (model.Value, v1.Warnings, error) {
			if q != nil {
				return q
			} else {
				return defaultQ
			}
		}(),
		qr: func() func(query string, r v1.Range) (model.Value, v1.Warnings, error) {
			if qr != nil {
				return qr
			} else {
				return defaultQr
			}
		}(),
	}
}

func (f *mockApi) Query(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error) {
	return f.q(query, ts)
}

func (f *mockApi) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, v1.Warnings, error) {
	return f.qr(query, r)
}

func (f *mockApi) Alerts(ctx context.Context) (v1.AlertsResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) AlertManagers(ctx context.Context) (v1.AlertManagersResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) CleanTombstones(ctx context.Context) error {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) Config(ctx context.Context) (v1.ConfigResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) Flags(ctx context.Context) (v1.FlagsResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) LabelNames(ctx context.Context, startTime time.Time, endTime time.Time) ([]string, v1.Warnings, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) LabelValues(ctx context.Context, label string, startTime time.Time, endTime time.Time) (model.LabelValues, v1.Warnings, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) Runtimeinfo(ctx context.Context) (v1.RuntimeinfoResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) ([]model.LabelSet, v1.Warnings, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) Snapshot(ctx context.Context, skipHead bool) (v1.SnapshotResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) Rules(ctx context.Context) (v1.RulesResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) Targets(ctx context.Context) (v1.TargetsResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) TargetsMetadata(ctx context.Context, matchTarget string, metric string, limit string) ([]v1.MetricMetadata, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) Metadata(ctx context.Context, metric string, limit string) (map[string][]v1.Metadata, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *mockApi) TSDB(ctx context.Context) (v1.TSDBResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
