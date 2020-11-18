module github.com/scylladb/scylla-operator-autoscaler

go 1.15

require (
	github.com/scylladb/go-log v0.0.4
	github.com/scylladb/scylla-operator v0.3.0
	github.com/spf13/cobra v0.0.5
	go.uber.org/zap v1.14.0
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.3
)
