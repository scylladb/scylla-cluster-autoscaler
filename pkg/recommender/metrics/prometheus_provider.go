package metrics

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/scylladb/go-log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type prometheusProvider struct {
	provider
	api v1.API
}

const (
	svcPort           = 9090
	maxQueriesInRange = 11000
)

func NewPrometheusProvider(api v1.API, logger log.Logger, defaultStep time.Duration) Provider {
	return &prometheusProvider{
		provider: provider{
			logger:      logger,
			defaultStep: defaultStep,
		},
		api: api,
	}
}

func NewPrometheusClient(ctx context.Context, c client.Client, selector map[string]string) (*api.Client, error) {
	svcList := &corev1.ServiceList{}
	err := c.List(ctx, svcList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(selector),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list prometheus services")
	}
	if len(svcList.Items) == 0 {
		return nil, errors.New("no prometheus server found")
	}

	svc := svcList.Items[0]
	addr := (&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, svcPort),
	}).String()

	promClient, err := api.NewClient(api.Config{Address: addr})
	if err != nil {
		return nil, errors.Wrap(err, "create prometheus client")
	}

	return &promClient, nil
}

func (p *prometheusProvider) Query(ctx context.Context, expression string) (bool, error) {
	result, warnings, err := p.api.Query(ctx, expression, time.Now())

	if err != nil {
		return false, errors.Wrap(err, "query")
	}

	if len(warnings) > 0 {
		p.logger.Error(ctx, "query", "warnings", warnings)
	}

	if result.Type() != model.ValVector {
		return false, errors.New("unhandled ValueType returned")
	}

	resultVector := result.(model.Vector)
	if resultVector.Len() == 0 {
		return false, errors.New("no results")
	}

	return resultVector[0].Value != 0, nil //TODO check all results instead of just the first one???
}

func (p *prometheusProvider) RangedQuery(ctx context.Context, expression string, duration time.Duration, argStep *time.Duration) (bool, error) {
	now := time.Now()
	step := p.defaultStep
	if argStep != nil {
		step = *argStep
	}
	if duration/step > maxQueriesInRange {
		step = duration/maxQueriesInRange + 1
	}

	result, warnings, err := p.api.QueryRange(ctx, expression, v1.Range{Start: now.Add(-duration), End: now, Step: step})

	if err != nil {
		return false, errors.Wrap(err, "ranged query")
	}

	if len(warnings) > 0 {
		p.logger.Error(ctx, "ranged query", "warnings", warnings)
	}

	if result.Type() != model.ValMatrix {
		return false, errors.New("unhandled ValueType returned")
	}

	resultMatrix := result.(model.Matrix)
	if resultMatrix.Len() == 0 || len(resultMatrix[0].Values) == 0 {
		return false, errors.New("no results")
	}

	status := true
	values := resultMatrix[0].Values //TODO check all results instead of just the first one???
	for i := range values {
		status = status && (values[i].Value != 0)
		if !status {
			break
		}
	}

	return status, nil
}
