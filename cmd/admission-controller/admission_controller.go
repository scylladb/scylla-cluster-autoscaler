package main

import (
	"context"
	"net/http"

	"github.com/scylladb/go-log"
	"github.com/spf13/cobra"
)

const (
	caMountPath = "/tmp/k8s-webhook-server/serving-certs/"
	tlsCert     = caMountPath + "tls.crt"
	tlsKey      = caMountPath + "tls.key"
	mutatePath  = "/mutate-scylla-scylladb-com-v1-scyllacluster"
	mutatePort  = ":443"
)

func newAdmissionControllerCmd(ctx context.Context, logger log.Logger) *cobra.Command {
	admissionControllerCmd := &cobra.Command{
		Use:   "admission-controller",
		Short: "Start the admission controller",
		Run: func(cmd *cobra.Command, args []string) {

			logger.Info(ctx, "Admission controller initiated")

			mux := http.NewServeMux()
			mux.Handle(mutatePath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				logger.Info(ctx, "Received request")
				for name, headers := range r.Header {
					for _, h := range headers {
						logger.Info(ctx, "Request", "name", name, "header", h)
					}
				}
			}))

			logger.Info(ctx, "Mux initiated")

			server := &http.Server{
				Addr:    mutatePort,
				Handler: mux,
			}

			logger.Info(ctx, "Starting listening to", "path", mutatePath)

			err := server.ListenAndServeTLS(tlsCert, tlsKey)

			logger.Error(ctx, "Server crashed", "error", err)

		},
	}

	return admissionControllerCmd
}
