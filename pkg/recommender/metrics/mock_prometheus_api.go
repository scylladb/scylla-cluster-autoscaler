package metrics

import (
	"context"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"time"
)

type MockApi struct {}

var Q = func(query string, ts time.Time) (model.Value, v1.Warnings, error) {
	return nil, []string{},  errors.New("Query function in mock_prometheus_api not implemented")
}

var Qr = func(query string, r v1.Range) (model.Value, v1.Warnings, error) {
	return nil, []string{}, errors.New("QueryRange function in mock_prometheus_api not implemented")
}

func (f *MockApi) Query(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error) {
	return Q(query, ts)
}

func (f *MockApi) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, v1.Warnings, error) {
	return Qr(query, r)
}

func (f *MockApi) Alerts(ctx context.Context) (v1.AlertsResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) AlertManagers(ctx context.Context) (v1.AlertManagersResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) CleanTombstones(ctx context.Context) error {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) Config(ctx context.Context) (v1.ConfigResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) Flags(ctx context.Context) (v1.FlagsResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) LabelNames(ctx context.Context, startTime time.Time, endTime time.Time) ([]string, v1.Warnings, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) LabelValues(ctx context.Context, label string, startTime time.Time, endTime time.Time) (model.LabelValues, v1.Warnings, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) Runtimeinfo(ctx context.Context) (v1.RuntimeinfoResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) ([]model.LabelSet, v1.Warnings, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) Snapshot(ctx context.Context, skipHead bool) (v1.SnapshotResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) Rules(ctx context.Context) (v1.RulesResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) Targets(ctx context.Context) (v1.TargetsResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) TargetsMetadata(ctx context.Context, matchTarget string, metric string, limit string) ([]v1.MetricMetadata, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) Metadata(ctx context.Context, metric string, limit string) (map[string][]v1.Metadata, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}
func (f *MockApi) TSDB(ctx context.Context) (v1.TSDBResult, error) {
	panic("Do not use this function. Implemented only for testing purposes.")
}