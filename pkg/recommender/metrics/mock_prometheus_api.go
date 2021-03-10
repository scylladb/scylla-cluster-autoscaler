package metrics

import (
	"context"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"math"
	"time"
)
// TODO add to vendor
type MockApi struct {}

var Q = func() (model.Value, v1.Warnings, error) {
	res := model.Vector{
		{
			Metric: model.Metric{
				"label_name_1.1": "label_value_1.1",
				"label_name_1.2": "label_value_1.2",
			},
			Value: model.SampleValue(1),
			Timestamp: model.Time(math.MinInt64),
		},
		{
			Metric: model.Metric{
				"label_name_2.1": "label_value_2.1",
				"label_name_2.2": "label_value_2.2",
			},
			Value: model.SampleValue(2),
			Timestamp: model.Time(math.MaxInt64),
		},
	}
	return res, []string{}, nil
}

var Qr = func() (model.Value, v1.Warnings, error) {
	return nil, []string{}, errors.New("halkooo QueryRange error")
}

func (f *MockApi) Query(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error) {
	return Q()
}

func (f *MockApi) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, v1.Warnings, error) {
	return Qr()
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