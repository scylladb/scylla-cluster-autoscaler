package main

import (
	"context"
	"os"

	"github.com/scylladb/go-log"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func newAdmissionControllerCmd(ctx context.Context, logger log.Logger) *cobra.Command {
	admissionControllerCmd := &cobra.Command{
		Use:   "admission-controller",
		Short: "Start the admission controller",
		Run: func(cmd *cobra.Command, args []string) {

			logger.Info(ctx, "Initiating Admission Controller")

			// setup a Manager
			logger.Info(ctx, "Setting up manager")
			mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
			if err != nil {
				logger.Error(ctx, "Unable to set up overall controller manager", err)
				os.Exit(1)
			}

			// setup webhooks
			logger.Info(ctx, "Setting up webhook server")
			webhookServer := mgr.GetWebhookServer()

			logger.Info(ctx, "Registering webhooks to the webhook server")
			webhookServer.Register("/mutate-scylla-scylladb-com-v1-scyllacluster", &webhook.Admission{Handler: &recommendationApplier{Client: mgr.GetClient(), logger: logger}})

			logger.Info(ctx, "Starting manager")
			if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
				logger.Error(ctx, "Unable to run manager", err)
				os.Exit(1)
			}
		},
	}

	return admissionControllerCmd
}
