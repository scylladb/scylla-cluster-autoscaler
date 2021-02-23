package metrics

import (
	"bytes"
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
}

func NewPrometheusProvider(ctx context.Context, c client.Client, logger log.Logger, metricsSelector map[string]string) (Provider, error) {
	promClient, err := discover(ctx, c, metricsSelector)
	if err != nil {
		return nil, err
	}

	return &prometheusProvider{
		provider: provider{
			api:    v1.NewAPI(*promClient),
			logger: logger,
		},
	}, nil
}

func discover(ctx context.Context, c client.Client, metricsSelector map[string]string) (*api.Client, error) {
	svcList := &corev1.ServiceList{}
	err := c.List(ctx, svcList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(metricsSelector),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list prometheus services")
	}
	if len(svcList.Items) == 0 {
		return nil, errors.Wrap(err, "no prometheus server found")
	}

	svc := svcList.Items[0]
	addr := (&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
	}).String()

	promClient, err := api.NewClient(api.Config{Address: addr})
	if err != nil {
		return nil, errors.Wrap(err, "create prometheus client")
	}

	return &promClient, nil
}

func mapToString(m map[string]string) (string, error) {
	b := new(bytes.Buffer)

	for key, value := range m {
		_, err := fmt.Fprintf(b, "%s=\"%s\",", key, value)
		if err != nil {
			return "", err
		}
	}

	return b.String(), nil
}

func (p *prometheusProvider) FetchMetric(ctx context.Context, metric string, labels map[string]string) (float64, error) {
	labelsStr, err := mapToString(labels)
	if err != nil {
		return 0, errors.Wrap(err, "convert labels map to string")
	}

	query := fmt.Sprintf("%s{%s}", metric, labelsStr)
	result, warnings, err := p.api.Query(ctx, query, time.Now())

	if err != nil {
		return 0, errors.Wrap(err, "query")
	}

	if len(warnings) > 0 {
		p.logger.Error(ctx, "query", "warnings", warnings)
	}

	resultVector := result.(model.Vector)
	if resultVector.Len() == 0 {
		return 0, errors.New("no results")
	}

	return float64(resultVector[0].Value), nil
}
