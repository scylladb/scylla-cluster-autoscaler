package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	v1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// admissionValidator checks whether requests from sources other than Updater change resources of ScyllaCluster
type admissionValidator struct {
	Client  client.Client
	decoder *admission.Decoder

	scyllaClient client.Client
	logger       log.Logger
}

const (
	updaterServiceAccountUsername = "system:serviceaccount:scylla-operator-autoscaler-system:scylla-operator-autoscaler-updater-service-account"
)

func validateClusterChanges(ctx context.Context, logger log.Logger, cluster *scyllav1.ScyllaCluster, oldCluster *scyllav1.ScyllaCluster, scas *v1alpha1.ScyllaClusterAutoscalerList) error {
	logger.Info(ctx, "starting validation of ScyllaCluster")

	for idx := range scas.Items {
		sca := &scas.Items[idx]

		if sca.Spec.TargetRef.Name != cluster.Name || sca.Spec.TargetRef.Namespace != cluster.Namespace {
			logger.Debug(ctx, "SCA different than SCA of this Admission Controller", "SCA name", sca.Spec.TargetRef.Name, "SCA namespace", sca.Spec.TargetRef.Namespace)
			continue
		}

		if *sca.Spec.UpdatePolicy.UpdateMode == v1alpha1.UpdateModeOff {
			logger.Debug(ctx, "SCA has 'off' update mode, skipping", "SCA name", sca.Spec.TargetRef.Name)
			continue
		}

		logger.Debug(ctx, "cluster has 'Auto' update mode")

		// check if user is changing resources administered by autoscaler
		for idr := range cluster.Spec.Datacenter.Racks {
			rack := cluster.Spec.Datacenter.Racks[idr]

			oldRack := v1.RackSpec{}

			for idrOld := range oldCluster.Spec.Datacenter.Racks {
				r := oldCluster.Spec.Datacenter.Racks[idrOld]
				if rack.Name == r.Name {
					oldRack = r
					break
				}
			}

			if rack.Members != oldRack.Members {
				return fmt.Errorf("changing members is forbidden while cluster is administered by autoscaler")
			}

			if rack.Storage.Capacity != oldRack.Storage.Capacity {
				return fmt.Errorf("changing storage.capacity is forbidden while cluster is administered by autoscaler")
			}

			if !rack.Resources.Requests.Cpu().Equal(*oldRack.Resources.Requests.Cpu()) {
				return fmt.Errorf("changing requests.cpu is forbidden while cluster is administered by autoscaler")
			}

			if !rack.Resources.Requests.Memory().Equal(*oldRack.Resources.Requests.Memory()) {
				return fmt.Errorf("changing requests.memory is forbidden while cluster is administered by autoscaler")
			}
		}
	}

	logger.Debug(ctx, "cluster change request successfully passed validation")

	return nil
}

func (av *admissionValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	cluster := &scyllav1.ScyllaCluster{}
	oldCluster := &scyllav1.ScyllaCluster{}
	var err error

	if len(req.OldObject.Raw) == 0 {
		return admission.Errored(http.StatusBadRequest, errors.New("there is no content to decode"))
	}
	if err = av.decoder.DecodeRaw(req.OldObject, oldCluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err = av.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	scas := &v1alpha1.ScyllaClusterAutoscalerList{}
	if err := av.scyllaClient.List(ctx, scas); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	av.logger.Debug(ctx, "SCAs fetched", "num", len(scas.Items))

	if req.AdmissionRequest.UserInfo.Username != updaterServiceAccountUsername {
		if err = validateClusterChanges(ctx, av.logger, cluster, oldCluster, scas); err != nil {
			return admission.Denied(err.Error())
		}
	} else {
		av.logger.Debug(ctx, "skipping validation for Updater request", "username", req.AdmissionRequest.UserInfo.Username)
	}

	return admission.Allowed("")
}

func (av *admissionValidator) InjectDecoder(d *admission.Decoder) error {
	av.decoder = d
	return nil
}
