package main

import (
	"context"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender"
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

			r, err := recommender.New(ctx, c, logger, metricsSelectorSet, metricsDefaultStep)
			if err != nil {
				logger.Fatal(ctx, "create recommender", "error", err)
				return
			}

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
