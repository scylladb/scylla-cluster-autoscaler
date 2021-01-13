package main

import (
	"context"
	"github.com/scylladb/go-log"
	"github.com/spf13/cobra"
)

const (
	caMountPath = "/tmp/k8s-webhook-server/serving-certs/"
	tlsCert     = caMountPath + "tls.crt"
	tlsKey      = caMountPath + "tls.key"
	mutatePath  = "/mutate-scylla-scylladb-com-v1-scyllacluster"
)

func newAdmissionControllerCmd(ctx context.Context, logger log.Logger) *cobra.Command {
	admissionControllerCmd := &cobra.Command{
		Use:   "admission-controller",
		Short: "Start the admission controller",
		Run: func(cmd *cobra.Command, args []string) {

		},
	}

	return admissionControllerCmd
}
