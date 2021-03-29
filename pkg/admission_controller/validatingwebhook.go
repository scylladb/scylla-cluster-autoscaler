package admission_controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// admissionValidator checks whether requests from sources other than Updater change resources of ScyllaCluster
type AdmissionValidator struct {
	Client  client.Client
	Decoder *admission.Decoder

	ScyllaClient                  client.Client
	Logger                        log.Logger
	UpdaterServiceAccountUsername string
	ScaledResources               []string
}

func validateClusterChanges(ctx context.Context, logger log.Logger, cluster, oldCluster *scyllav1.ScyllaCluster,
	scas *v1alpha1.ScyllaClusterAutoscalerList, scaledResources []string) error {

	logger.Info(ctx, "starting validation of ScyllaCluster")

	for _, sca := range scas.Items {

		if sca.Spec.TargetRef.Name != cluster.Name || sca.Spec.TargetRef.Namespace != cluster.Namespace {
			logger.Debug(ctx, "SCA different than SCA of this Admission Controller", "SCA name", sca.Spec.TargetRef.Name, "SCA namespace", sca.Spec.TargetRef.Namespace)
			continue
		}

		if sca.Spec.UpdatePolicy.UpdateMode == v1alpha1.UpdateModeOff {
			logger.Debug(ctx, "SCA has 'off' update mode, skipping", "SCA name", sca.Spec.TargetRef.Name)
			continue
		}

		logger.Debug(ctx, "cluster has 'Auto' update mode")

		// check if user is changing resources administered by autoscaler
		for _, rack := range cluster.Spec.Datacenter.Racks {

			oldRack := scyllav1.RackSpec{}
			oldRackAssigned := false

			for idrOld := range oldCluster.Spec.Datacenter.Racks {
				r := oldCluster.Spec.Datacenter.Racks[idrOld]
				if rack.Name == r.Name {
					oldRack = r
					oldRackAssigned = true
					break
				}
			}

			if !oldRackAssigned {
				logger.Debug(ctx, "given rack wasn't found in older cluster", "rack", rack.Name)
				continue
			}

			if rack.Members != oldRack.Members {
				return fmt.Errorf("changing members is forbidden while cluster is administered by autoscaler")
			}

			for _, resourceName := range scaledResources {
				if !rack.Resources.Requests[v1.ResourceName(resourceName)].Equal(oldRack.Resources.Requests[v1.ResourceName(resourceName)]) {
					return fmt.Errorf("changing requests.%s is forbidden while cluster is administered by autoscaler", resourceName)
				}

				if !rack.Resources.Limits[v1.ResourceName(resourceName)].Equal(oldRack.Resources.Limits[v1.ResourceName(resourceName)]) {
					return fmt.Errorf("changing limits.%s is forbidden while cluster is administered by autoscaler", resourceName)
				}
			}

		}
	}

	logger.Debug(ctx, "cluster change request successfully passed validation")

	return nil
}

func (av *AdmissionValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	cluster := &scyllav1.ScyllaCluster{}
	oldCluster := &scyllav1.ScyllaCluster{}
	var err error

	if len(req.OldObject.Raw) == 0 {
		return admission.Errored(http.StatusBadRequest, errors.New("there is no content to decode"))
	}
	if err = av.Decoder.DecodeRaw(req.OldObject, oldCluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err = av.Decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	scas := &v1alpha1.ScyllaClusterAutoscalerList{}
	if err := av.ScyllaClient.List(ctx, scas); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	av.Logger.Debug(ctx, "SCAs fetched", "num", len(scas.Items))

	if req.AdmissionRequest.UserInfo.Username != av.UpdaterServiceAccountUsername {
		if err = validateClusterChanges(ctx, av.Logger, cluster, oldCluster, scas, av.ScaledResources); err != nil {
			return admission.Denied(err.Error())
		}
	} else {
		av.Logger.Debug(ctx, "skipping validation for Updater request", "username", req.AdmissionRequest.UserInfo.Username)
	}

	return admission.Allowed("")
}

func (av *AdmissionValidator) InjectDecoder(d *admission.Decoder) error {
	av.Decoder = d
	return nil
}
