package main

import (
	"os"
	"time"

	"context"
	"github.com/scylladb/go-log"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func newOperatorAutoscalerCmd(ctx context.Context, logger log.Logger, level zap.AtomicLevel) *cobra.Command {
	var operatorAutoscalerCmd = &cobra.Command{
		Use:   "operator-autoscaler",
		Short: "Start the scylla operator autoscaler",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info(ctx, "Operator autoscaler initiated")

			c, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
			if err != nil {
				logger.Error(ctx, "failed to create a client")
				os.Exit(1)
			}

			for {
				time.Sleep(30 * time.Second)

				// 1. List Cluster resources

				//clusterList := &scyllav1alpha1.ClusterList{}
				//
				//err = c.List(ctx, clusterList, client.InNamespace("scylla"))
				//if err != nil {
				//	logger.Error(ctx, "failed to list clusters in namespace scylla", "error", err)
				//	continue
				//}
				//
				//logger.Info(ctx, "scylla cluster list:", "list", clusterList.Items)

				// 2. Update number of members in cluster
				cluster := &scyllav1alpha1.Cluster{}
				err = c.Get(ctx, client.ObjectKey{
					Namespace: "scylla",
					Name:      "simple-cluster"}, cluster)
				if err != nil {
					logger.Error(ctx, "failed to get cluster 'simple-cluster' in namespace 'scylla'", "error", err)
					continue
				}

				racks := cluster.Spec.Datacenter.Racks
				idx := -1
				for i, r := range racks {
					if r.Name == "us-east-1a" {
						idx = i
					}
				}
				if idx == -1 {
					logger.Error(ctx, "rack 'us-east-1a' not found in the datacenter")
					continue
				}

				racks[idx].Members = 3
				err = c.Update(ctx, cluster)
				if err != nil {
					logger.Error(ctx, "failed to change number of members to 3", "error", err)
					continue
				}

				logger.Info(ctx, "changed number of members to 3")
			}
		},
	}

	return operatorAutoscalerCmd
}
