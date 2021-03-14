package main

import (
	"context"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/updater"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().DurationP("interval", "i", 1*time.Minute, "Update interval")
}

func newUpdaterCmd(ctx context.Context, logger log.Logger) *cobra.Command {
	updaterCmd := &cobra.Command{
		Use:   "updater",
		Short: "Start the updater",
		Run: func(cmd *cobra.Command, args []string) {
			updateInterval, err := cmd.Flags().GetDuration("interval")
			if err != nil {
				logger.Fatal(ctx, "get update interval", "err", err)
			}

			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
				Scheme: scheme,
			})
			if err != nil {
				logger.Fatal(ctx, "create manager", "error", err)
			}

			c, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
			if err != nil {
				logger.Fatal(ctx, "get dynamic client", "error", err)
			}

			u := updater.NewUpdater(c, logger)
			ticker := time.Tick(updateInterval)
			for range ticker {
				if err = u.RunOnce(ctx); err != nil {
					logger.Error(ctx, "updater run once", "error", err)
				}
			}
		},
	}

	addFlags(updaterCmd)
	return updaterCmd
}
