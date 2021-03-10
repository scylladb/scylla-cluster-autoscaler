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
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	logger.Info(ctx, "initiating Admission Controller")

	// setup a Manager
	logger.Info(ctx, "setting up manager")
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Error(ctx, "unable to set up overall controller manager", "err", err)
		os.Exit(1)
	}

	// setup a client
	client, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})

	// setup webhooks
	logger.Info(ctx, "setting up webhook server")
	webhookServer := mgr.GetWebhookServer()

	logger.Info(ctx, "registering webhooks to the webhook server")
	webhookServer.Register("/validate-scylla-scylladb-com-v1-scyllacluster", &webhook.Admission{
		Handler: &admissionValidator{Client: mgr.GetClient(), logger: logger, scyllaClient: client},
	})

	logger.Info(ctx, "starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		logger.Error(ctx, "unable to run manager", "err", err)
		os.Exit(1)
	}
}
