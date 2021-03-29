package main

import (
	"context"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var (
	metricsInterval    time.Duration
	metricsSelectorSet map[string]string
	metricsDefaultStep time.Duration
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().DurationVarP(&metricsInterval, "interval", "i", time.Minute, "Running interval")
	cmd.Flags().StringToStringVar(&metricsSelectorSet, "metrics-selector-set", make(map[string]string, 0), "Label selector set for metrics server discovery")
	cmd.Flags().DurationVar(&metricsDefaultStep, "metrics-default-step", time.Minute, "Metrics ranged queries' default step")
}

func newRecommenderCmd(ctx context.Context, logger log.Logger) *cobra.Command {
	recommenderCmd := &cobra.Command{
		Use:   "recommender",
		Short: "Start the recommender",
		Run: func(cmd *cobra.Command, args []string) {
			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
				Scheme: scheme,
			})
			if err != nil {
				logger.Fatal(ctx, "create manager", "error", err)
				return
			}

			c, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
			if err != nil {
				logger.Fatal(ctx, "get dynamic client", "error", err)
				return
			}

			pc, err := metrics.NewPrometheusClient(ctx, c, metricsSelectorSet)
			if err != nil {
				logger.Fatal(ctx, "create prometheus client", "error", err)
				return
			}

			pp := metrics.NewPrometheusProvider(v1.NewAPI(*pc), logger, metricsDefaultStep)

			r := recommender.New(c, pp, logger)

			ticker := time.Tick(metricsInterval)
			for range ticker {
				if err := r.RunOnce(ctx); err != nil {
					logger.Error(ctx, "running once", "error", err)
				}
			}
		},
	}

	addFlags(recommenderCmd)
	return recommenderCmd
}
