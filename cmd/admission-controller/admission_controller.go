package main

import (
	"context"
	"os"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/admission_controller"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	updaterServiceAccountUsername string
	scaledResources               []string
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&updaterServiceAccountUsername,
		"updater-service-account-username",
		"system:serviceaccount:scylla-operator-autoscaler-system:scylla-operator-autoscaler-updater-service-account",
		"Updater service account username, used for filtering admission requests which are sent from the outside of autoscaler",
	)
	cmd.Flags().StringSliceVar(
		&scaledResources,
		"scaled-resources",
		[]string{"cpu"},
		"Scaled resources names, separated by commas",
	)
}

func newAdmissionControllerCmd(ctx context.Context, logger log.Logger) *cobra.Command {
	admissionControllerCmd := &cobra.Command{
		Use:   "admission-controller",
		Short: "Start the admission controller",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info(ctx, "initiating Admission Controller")

			// manager setup
			logger.Info(ctx, "setting up manager")
			mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
			if err != nil {
				logger.Error(ctx, "unable to set up overall controller manager", "err", err)
				os.Exit(1)
			}

			// client setup
			client, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})

			// webhook setup
			logger.Info(ctx, "setting up webhook server")
			webhookServer := mgr.GetWebhookServer()

			logger.Info(ctx, "registering webhooks to the webhook server")
			webhookServer.Register("/validate-scylla-scylladb-com-v1-scyllacluster", &webhook.Admission{
				Handler: &admission_controller.AdmissionValidator{
					Client:                        mgr.GetClient(),
					Logger:                        logger,
					ScyllaClient:                  client,
					UpdaterServiceAccountUsername: updaterServiceAccountUsername,
					ScaledResources:               scaledResources,
				},
			})

			logger.Info(ctx, "starting manager")
			if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
				logger.Error(ctx, "unable to run manager", "err", err)
				os.Exit(1)
			}
		},
	}

	addFlags(admissionControllerCmd)
	return admissionControllerCmd
}
