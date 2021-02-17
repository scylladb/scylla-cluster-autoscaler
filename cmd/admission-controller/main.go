package main

import (
	"context"
	"os"

	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = scyllav1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})

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
	webhookServer.Register("/mutate-scylla-scylladb-com-v1-scyllacluster", &webhook.Admission{
		Handler: &recommendationApplier{Client: mgr.GetClient(), logger: logger},
	})

	logger.Info(ctx, "Starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		logger.Error(ctx, "Unable to run manager", err)
		os.Exit(1)
	}
}
