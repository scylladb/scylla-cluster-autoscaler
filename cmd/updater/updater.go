package main

import (
	"context"
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
	cmd.Flags().DurationP("interval", "i", 2*time.Minute, "Update interval")
}

func getDataCenterRecommendations(sca *v1alpha1.ScyllaClusterAutoscaler) []v1alpha1.DataCenterRecommendations {
	if sca.Status.Recommendations == nil {
		return nil
	}

	dcRecs := sca.Status.Recommendations.DataCenterRecommendations
	if dcRecs == nil || len(dcRecs) == 0 {
		return nil
	}

	return dcRecs
}

func getRackRecommendations(dataCenterName string,
	dcRecs []v1alpha1.DataCenterRecommendations) []v1alpha1.RackRecommendations {
	for idx := range dcRecs {
		if dcRecs[idx].Name == dataCenterName {
			if dcRecs[idx].RackRecommendations == nil {
				return nil
			} else {
				return dcRecs[idx].RackRecommendations
			}
		}
	}

	return nil
}

func findRack(rackName string, racks []scyllav1alpha1.RackSpec) *scyllav1alpha1.RackSpec {
	for idx := range racks {
		if rackName == racks[idx].Name {
			return &racks[idx]
		}
	}

	return nil
}

func newUpdaterCmd(ctx context.Context, logger log.Logger, level zap.AtomicLevel) *cobra.Command {
	updaterCmd := &cobra.Command{
		Use:   "updater",
		Short: "Start the updater",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info(ctx, "Updater initiated")

			updateInterval, _ := cmd.Flags().GetDuration("interval")

			ticker := time.Tick(updateInterval)
			for range ticker {
				logger.Debug(ctx, "Updater running once")

				c, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
				if err != nil {
					logger.Error(ctx, "failed to create a client")
					os.Exit(1)
				}

				scas := &v1alpha1.ScyllaClusterAutoscalerList{}
				err = c.List(ctx, scas)
				if err != nil {
					logger.Error(ctx, "failed to get SCAs", "error", err)
					continue
				} else {
					logger.Debug(ctx, "SCAs fetched", "num", len(scas.Items))
				}

				for idx := range scas.Items {
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

					dcRecs := getDataCenterRecommendations(sca)
					if dcRecs == nil {
						logger.Debug(ctx, "no recommendations for cluster", "cluster", cluster.Name)
						continue
					}

					dataCenterName := cluster.Spec.Datacenter.Name

					rackRecs := getRackRecommendations(dataCenterName, dcRecs)
					if rackRecs == nil {
						logger.Debug(ctx, "no recommendations for data center", "data center",
							dataCenterName)
						continue
					}

					for j := range rackRecs {
						rackRec := &rackRecs[j]

						if rackRec.Members == nil {
							logger.Debug(ctx, "no members recommendation for rack",
								"rack", rackRec.Name)
							continue
						}

						rack := findRack(rackRec.Name, cluster.Spec.Datacenter.Racks)
						if rack == nil {
							logger.Debug(ctx, "could not find rack matching recommendation",
								"rack", rackRec.Name)
							continue
						}

						rack.Members = rackRec.Members.Target

						err = c.Update(ctx, cluster)
						if err != nil {
							logger.Error(ctx, "Cluster update error",
								"cluster", cluster.Name, "error", err)
						}

						logger.Debug(ctx, "rack updated", "rack", rackRec.Name)
					}
				}
			}
		},
	}

	addFlags(updaterCmd)
	return updaterCmd
}
