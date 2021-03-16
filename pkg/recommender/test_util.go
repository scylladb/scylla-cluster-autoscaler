package recommender

import (
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics"
)

func GetEmptyRecommender(l log.Logger) *recommender {
	return &recommender{
		metricsProvider: metrics.GetEmptyPrometheusProvider(l),
		logger:          l,
	}
}

func (r *recommender) SetMetricsProviderFakeAPI(api metrics.MockApi) {
	r.metricsProvider.SetApi(&api)
}
