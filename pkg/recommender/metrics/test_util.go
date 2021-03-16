package metrics

import (
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/scylladb/go-log"
	"time"
)

func GetEmptyPrometheusProvider(logger log.Logger) Provider {
	return &prometheusProvider{
		provider: provider{
			logger:      logger,
			defaultStep: time.Minute,
		},
		api: nil,
	}
}

func (p *prometheusProvider) SetApi(api v1.API) {
	p.api = api
}