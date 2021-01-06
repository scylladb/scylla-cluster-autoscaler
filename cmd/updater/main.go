package main

import (
	"context"
	"github.com/scylladb/go-log"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = scyllav1alpha1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})

	var rootCmd = &cobra.Command{}
	rootCmd.AddCommand(
		newUpdaterCmd(ctx, logger, atom),
	)
	if err := rootCmd.Execute(); err != nil {
		logger.Error(context.Background(), "Root command: a fatal error occured", "error", err)
	}
}
