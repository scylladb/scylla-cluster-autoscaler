package recommender

import (
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics"
)

/*func SetMetricsProviderMockApi(r *recommender, MockApi metrics.MockApi) {
	metrics.SetApi(r.metricsProvider, MockApi)
}*/

/*func QueryMetricsProvider(r *recommender, ctx context.Context, expression string) {
	r.metricsProvider.Query(ctx, expression)
}*/

/*func GetEmptyRecommender() *recommender {
	return &recommender{}
}*/

func GetEmptyRecommender(l log.Logger) *recommender {
	return &recommender{
		metricsProvider: metrics.GetEmptyPrometheusProvider(l),
		logger:          l,
	}
}

func (r *recommender) SetMetricsProviderFakeAPI(api metrics.MockApi) {
	r.metricsProvider.SetApi(&api)
}


