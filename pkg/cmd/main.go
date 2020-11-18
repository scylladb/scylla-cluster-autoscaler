package main

import (
	"context"
	"github.com/scylladb/go-log"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = scyllav1alpha1.AddToScheme(scheme)
}

func main() {
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})

	var rootCmd = &cobra.Command{}
	rootCmd.AddCommand(
		newOperatorAutoscalerCmd(ctx, logger, atom),
	)
	if err := rootCmd.Execute(); err != nil {
		logger.Error(context.Background(), "Root command: a fatal error occured", "error", err)
	}
}
