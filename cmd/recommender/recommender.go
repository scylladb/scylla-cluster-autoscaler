package main

import (
	"context"
	"encoding/json"
	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"time"
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().DurationP("interval", "i", 30*time.Second, "Metrics fetching interval")
	cmd.Flags().StringP("prometheusAddress", "p", "", "Prometheus metrics server address")
	_ = cmd.MarkFlagRequired("prometheusAddress")
}

func newRecommenderCmd(ctx context.Context, logger log.Logger, level zap.AtomicLevel) *cobra.Command {
	recommenderCmd := &cobra.Command{
		Use:   "recommender",
		Short: "Start the recommender",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info(ctx, "Recommender initiated")

			metricsInterval, _ := cmd.Flags().GetDuration("interval")
			prometheusAddress, _ := cmd.Flags().GetString("prometheusAddress")

			ticker := time.Tick(metricsInterval)
			for range ticker {
				logger.Debug(ctx, "Recommender running once")
				c, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
				if err != nil {
					logger.Error(ctx, "failed to create a client")
					os.Exit(1)
				}

				scas := &v1alpha1.ScyllaClusterAutoscalerList{}
				err = c.List(ctx, scas)
				if err != nil {
					logger.Error(ctx, "failed to get SCAs", "error", err)
				} else {
					logger.Debug(ctx, "SCAs fetched", "num", len(scas.Items))
				}

				for idx, _ := range scas.Items {
					sca := &scas.Items[idx]
					targetRef := sca.Spec.TargetRef

					cluster := &scyllav1alpha1.Cluster{}
					err = c.Get(ctx, client.ObjectKey{
						Namespace: targetRef.Namespace,
						Name:      targetRef.Name,
					}, cluster)

					if err != nil {
						logger.Error(ctx, "Error in fetching a referenced cluster", "error", err)
						continue
					}

					logger.Debug(ctx, "fetched cluster", "cluster", cluster.Name)

					prometheusClient, err := prometheusApi.NewClient(prometheusApi.Config{
						Address: prometheusAddress,
					})

					if err != nil {
						logger.Error(ctx, "Error in creating Prometheus client", "error", err)
						continue
					}

					prometheusv1api := prometheusv1.NewAPI(prometheusClient)

					recommendations := &v1alpha1.ScyllaClusterRecommendations{DataCenterRecommendations: make([]v1alpha1.DataCenterRecommendations, 0)}
					rackRecommendations := make([]v1alpha1.RackRecommendations, 0)

					for _, rack := range cluster.Spec.Datacenter.Racks {
						logger.Debug(ctx, "querying for rack:", "rack", rack.Name)

						result, warnings, err := prometheusv1api.Query(ctx,
							"avg(scylla_reactor_utilization{cluster=\""+cluster.Name+
								"\", dc=\""+cluster.Spec.Datacenter.Name+
								"\", rack=\""+rack.Name+"\"})", time.Now())

						if err != nil {
							logger.Error(ctx, "Error querying prometheus", "error", err)
							continue
						}

						if len(warnings) > 0 {
							logger.Error(ctx, "Warnings:", "warnings", warnings)
						}

						resultVector := result.(model.Vector)
						if resultVector.Len() == 0 {
							logger.Error(ctx, "no results returned")
							continue
						}

						util := resultVector[0].Value

						logger.Debug(ctx, "CPU utilisation", "rack", rack.Name, "util", util)

						newMembers := rack.Members
						if util > 70 {
							newMembers += 1
						}

						rackRecommendations = append(rackRecommendations, v1alpha1.RackRecommendations{Name: rack.Name, Members: &v1alpha1.RecommendedRackMembers{Target: newMembers}})
					}

					recommendations.DataCenterRecommendations = append(recommendations.DataCenterRecommendations,
						v1alpha1.DataCenterRecommendations{Name: cluster.Name, RackRecommendations: rackRecommendations})

					str, err := json.Marshal(recommendations)
					if err != nil {
						logger.Error(ctx, "recommendations marshall error", "error", err)
						continue
					}
					logger.Info(ctx, "recommendations", "sca", sca.Name, "json", string(str))

					sca.Status.Recommendations = recommendations
					err = c.Status().Update(ctx, sca)

					if err != nil {
						logger.Error(ctx, "SCA status update error", "error", err)
					}
				}
			}
		},
	}

	addFlags(recommenderCmd)
	return recommenderCmd
}
