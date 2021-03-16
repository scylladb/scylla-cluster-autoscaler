package recommender

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics"
	rutil "github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/util"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/util"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type Recommender interface {
	RunOnce(ctx context.Context) error
	/*SetMetricsProviderFakeAPI(api metrics.MockApi)
	ExportedQueryOnMetricsProvider(ctx context.Context, expression string) (bool, error)*/
}

type recommender struct {
	client          client.Client
	logger          log.Logger
	metricsProvider metrics.Provider
}

func New(ctx context.Context, c client.Client, logger log.Logger, metricsSelector map[string]string, metricsDefaultStep time.Duration) (Recommender, error) {
	p, err := metrics.NewPrometheusProvider(ctx, c, logger, metricsSelector, metricsDefaultStep)
	if err != nil {
		return nil, errors.Wrap(err, "create metrics provider")
	}

	return &recommender{
		client:          c,
		logger:          logger,
		metricsProvider: p,
	}, nil
}

func (r *recommender) RunOnce(ctx context.Context) error {
	scas, err := r.fetchSCAs(ctx)
	if err != nil {
		return errors.Wrap(err, "fetch SCAs")
	}

	for _, sca := range scas.Items {
		targetRef := sca.Spec.TargetRef
		sc, err := r.fetchScyllaCluster(ctx, targetRef.Name, targetRef.Namespace)
		if err != nil {
			r.logger.Error(ctx, "fetch target", "sca", sca.Name, "namespace", sca.Namespace, "error", err)
			r.updateSCAStatus(ctx, &sca, v1alpha1.UpdateStatusTargetFetchFail, nil)
			continue
		}

		if !isScyllaClusterReady(sc) {
			r.logger.Error(ctx, "target readiness check", "sca", sca.Name, "namespace", sca.Namespace)
			r.updateSCAStatus(ctx, &sca, v1alpha1.UpdateStatusTargetNotReady, nil)
			continue
		}

		recommendations, err := r.getScyllaClusterRecommendations(ctx, sc, sca.Spec.ScalingPolicy)
		status := v1alpha1.UpdateStatusOk
		if err != nil {
			r.logger.Error(ctx, "prepare recommendations", "sca", sca.Name, "namespace", sca.Namespace, "error", err)
			status = v1alpha1.UpdateStatusRecommendationsFail
		}
		r.updateSCAStatus(ctx, &sca, status, recommendations)
	}

	return nil
}

func (r *recommender) ExportedQueryOnMetricsProvider(ctx context.Context, expression string) (bool, error) {
	return r.metricsProvider.Query(ctx, expression)
}

func (r *recommender) ExportedRangedQueryOnMetricsProvider(ctx context.Context, expression string, duration time.Duration, argStep *time.Duration) (bool, error) {
	return r.metricsProvider.RangedQuery(ctx, expression, duration, argStep)
}

func (r *recommender) ExportedGetRackRecommendations(ctx context.Context, rack *scyllav1.RackSpec, scalingPolicy *v1alpha1.RackScalingPolicy) (*v1alpha1.RackRecommendations, error) {
	return r.getRackRecommendations(ctx, rack, scalingPolicy)
}

func (r *recommender) updateSCAStatus(ctx context.Context, sca *v1alpha1.ScyllaClusterAutoscaler, status v1alpha1.UpdateStatus, recommendations *v1alpha1.ScyllaClusterRecommendations) {
	sca.Status.LastUpdated = metav1.NewTime(time.Now())
	sca.Status.UpdateStatus = &status
	sca.Status.Recommendations = recommendations

	err := r.client.Status().Update(ctx, sca)
	if err != nil {
		r.logger.Error(ctx, "SCA status update", "sca", sca.Name, "namespace", sca.Namespace, "error", err)
	}
}

func (r *recommender) fetchSCAs(ctx context.Context) (*v1alpha1.ScyllaClusterAutoscalerList, error) {
	scas := &v1alpha1.ScyllaClusterAutoscalerList{}
	if err := r.client.List(ctx, scas); err != nil {
		return nil, err
	}

	return scas, nil
}

func (r *recommender) fetchScyllaCluster(ctx context.Context, name, namespace string) (*scyllav1.ScyllaCluster, error) {
	sc := &scyllav1.ScyllaCluster{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, sc); err != nil {
		return nil, err
	}

	return sc, nil
}

func isScyllaClusterReady(sc *scyllav1.ScyllaCluster) bool {
	for _, rack := range sc.Spec.Datacenter.Racks {
		rackStatus, found := sc.Status.Racks[rack.Name]
		if !found || rackStatus.Members != rackStatus.ReadyMembers {
			return false
		}
	}

	return true
}

func (r *recommender) getScyllaClusterRecommendations(ctx context.Context, sc *scyllav1.ScyllaCluster, scalingPolicy *v1alpha1.ScalingPolicy) (*v1alpha1.ScyllaClusterRecommendations, error) {
	var datacenterRecommendations []v1alpha1.DatacenterRecommendations
	datacenter := sc.Spec.Datacenter
	for _, datacenterScalingPolicy := range scalingPolicy.Datacenters {
		if datacenterScalingPolicy.Name != datacenter.Name {
			return nil, errors.New(fmt.Sprintf("datacenter \"%s\" not found", datacenterScalingPolicy.Name))
		}

		recommendations, err := r.getDatacenterRecommendations(ctx, &datacenter, &datacenterScalingPolicy)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("datacenter \"%s\"", datacenter.Name))
		}
		if recommendations != nil {
			datacenterRecommendations = append(datacenterRecommendations, *recommendations)
		}
	}

	if len(datacenterRecommendations) > 0 {
		return &v1alpha1.ScyllaClusterRecommendations{DatacenterRecommendations: datacenterRecommendations}, nil
	}

	return nil, nil
}

func (r *recommender) getDatacenterRecommendations(ctx context.Context, datacenter *scyllav1.DatacenterSpec, scalingPolicy *v1alpha1.DatacenterScalingPolicy) (*v1alpha1.DatacenterRecommendations, error) {
	var rackRecommendations []v1alpha1.RackRecommendations
	for _, rackScalingPolicy := range scalingPolicy.RackScalingPolicies {
		var rack scyllav1.RackSpec

		found := false
		for i := range datacenter.Racks {
			if datacenter.Racks[i].Name == rackScalingPolicy.Name {
				rack = datacenter.Racks[i]
				found = true
				break
			}
		}

		if !found {
			return nil, errors.New(fmt.Sprintf("rack \"%s\" not found", rackScalingPolicy.Name))
		}

		recommendations, err := r.getRackRecommendations(ctx, &rack, &rackScalingPolicy)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("rack \"%s\"", rack.Name))
		}
		if recommendations != nil {
			rackRecommendations = append(rackRecommendations, *recommendations)
		}
	}

	if len(rackRecommendations) > 0 {
		return &v1alpha1.DatacenterRecommendations{Name: datacenter.Name, RackRecommendations: rackRecommendations}, nil
	}

	return nil, nil
}

func (r *recommender) getRackRecommendations(ctx context.Context, rack *scyllav1.RackSpec, scalingPolicy *v1alpha1.RackScalingPolicy) (*v1alpha1.RackRecommendations, error) {
	if scalingPolicy == nil {
		return nil, errors.New("scaling policy not defined")
	} else if rack == nil {
		return nil, errors.New("rack spec not defined")
	}
	var err error
	var priority int32 = math.MaxInt32
	members := rack.Members
	resources := rack.Resources

	applied := false
	for _, rule := range scalingPolicy.ScalingRules {
		if rule.Priority >= priority {
			continue // TODO solve conflicting priorities, i.e. two rules with equal priorities???
		}

		var res bool
		if rule.For != nil {
			var step *time.Duration = nil
			if rule.Step != nil {
				step = &rule.Step.Duration
			}
			res, err = r.metricsProvider.RangedQuery(ctx, rule.Expression, rule.For.Duration, step)
		} else {
			res, err = r.metricsProvider.Query(ctx, rule.Expression)
		}

		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("rule \"%s\"", rule.Name))
		}

		if !res {
			continue
		}

		if rule.ScalingMode == v1alpha1.ScalingModeHorizontal {
			var min, max *int32 = nil, nil
			if scalingPolicy.MemberPolicy != nil {
				max = scalingPolicy.MemberPolicy.MaxAllowed
				min = scalingPolicy.MemberPolicy.MinAllowed
			}

			members = rutil.CalculateMembers(rack.Members, min, max, rule.ScalingFactor)
			resources = rack.Resources
		} else {
			if rack.Resources.Requests == nil || rack.Resources.Requests.Cpu() == nil {
				return nil, errors.New("cpu requests undefined")
			}

			var min, max *resource.Quantity = nil, nil
			if scalingPolicy.ResourcePolicy != nil {
				min = scalingPolicy.ResourcePolicy.MinAllowedCpu
				max = scalingPolicy.ResourcePolicy.MaxAllowedCpu
			}
			resources.Requests[corev1.ResourceCPU] = rutil.CalculateCPU(rack.Resources.Requests.Cpu(), min, max, rule.ScalingFactor)

			if rack.Resources.Limits != nil && rack.Resources.Limits.Cpu() != nil {
				if scalingPolicy.ResourcePolicy.RackControlledValues == v1alpha1.RackControlledValuesRequestsAndLimits {
					resources.Limits[corev1.ResourceCPU] = rutil.CalculateCPU(rack.Resources.Limits.Cpu(), min, max, rule.ScalingFactor)
				} else {
					resources.Requests[corev1.ResourceCPU] = util.MinQuantity(resources.Requests[corev1.ResourceCPU], resources.Limits[corev1.ResourceCPU])
				}
			}
			members = rack.Members
		}

		priority = rule.Priority
		applied = true
	}

	if applied {
		return &v1alpha1.RackRecommendations{Name: rack.Name, Members: &members, Resources: &resources}, nil
	}

	return nil, nil
}
