package recommender

import (
	"context"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics"
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
}

type recommender struct {
	client          client.Client
	logger          log.Logger
	metricsProvider metrics.Provider
}

func New(c client.Client, provider metrics.Provider, logger log.Logger) Recommender {
	return &recommender{
		client:          c,
		logger:          logger,
		metricsProvider: provider,
	}
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

func (r *recommender) updateSCAStatus(ctx context.Context, sca *v1alpha1.ScyllaClusterAutoscaler, status v1alpha1.UpdateStatus, recommendations *v1alpha1.ScyllaClusterRecommendations) {
	sca.Status.LastUpdated = metav1.NewTime(time.Now().UTC())
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
		if !found || rack.Members != rackStatus.ReadyMembers {
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
			return nil, errors.Errorf("datacenter \"%s\" not found", datacenterScalingPolicy.Name)
		}

		recommendations, err := r.getDatacenterRecommendations(ctx, &datacenter, &datacenterScalingPolicy)
		if err != nil {
			return nil, errors.Wrapf(err, "datacenter \"%s\"", datacenter.Name)
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
			return nil, errors.Errorf("rack \"%s\" not found", rackScalingPolicy.Name)
		}

		recommendations, err := r.getRackRecommendations(ctx, &rack, &rackScalingPolicy)
		if err != nil {
			return nil, errors.Wrapf(err, "rack \"%s\"", rack.Name)
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
			return nil, errors.Wrapf(err, "rule \"%s\"", rule.Name)
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

			members = CalculateMembers(rack.Members, min, max, rule.ScalingFactor)
			resources = rack.Resources
		} else {
			if rack.Resources.Requests == nil || rack.Resources.Requests.Cpu() == nil {
				return nil, errors.Errorf("cpu requests undefined")
			}

			var min, max *resource.Quantity = nil, nil
			if scalingPolicy.ResourcePolicy != nil {
				min = scalingPolicy.ResourcePolicy.MinAllowedCpu
				max = scalingPolicy.ResourcePolicy.MaxAllowedCpu
			}
			resources.Requests[corev1.ResourceCPU] = CalculateCPU(rack.Resources.Requests.Cpu(), min, max, rule.ScalingFactor)

			if rack.Resources.Limits != nil && rack.Resources.Limits.Cpu() != nil {
				if scalingPolicy.ResourcePolicy.RackControlledValues == v1alpha1.RackControlledValuesRequestsAndLimits {
					resources.Limits[corev1.ResourceCPU] = CalculateCPU(rack.Resources.Limits.Cpu(), min, max, rule.ScalingFactor)
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

func CalculateMembers(current int32, min, max *int32, factor float64) int32 {
	var val int32
	// if scaled current will overflow int32
	if float64(current) >= float64(math.MaxInt32)/factor {
		// then set max value
		val = math.MaxInt32
	} else {
		val = int32(factor * float64(current))
	}

	if max != nil {
		val = util.MinInt32(val, *max)
	}

	if min != nil {
		val = util.MaxInt32(val, *min)
	}

	return val
}

func CalculateCPU(current, min, max *resource.Quantity, factor float64) resource.Quantity {
	var val resource.Quantity

	// TODO keep original scale???
	// TODO check if this makes any sense

	// if MilliValue won't overflow int64 and MilliValue*factor won't overflow int64
	if current.Value() <= (math.MaxInt64/1000) && float64(current.MilliValue()) <= math.MaxInt64/factor {
		// then MilliValue is scaled i.e. val = current.MilliValue() * factor
		val = *resource.NewMilliQuantity(int64(factor*float64(current.MilliValue())), current.Format)

		// else if MilliValue will overflow int64, but Value*factor won't overflow int64
	} else if float64(current.Value()) <= math.MaxInt64/factor {
		// then Value is scaled i.e. val = current.Value() * factor
		val = *resource.NewQuantity(int64(factor*float64(current.Value())), current.Format)

		// else if both will overflow int64
	} else {
		// then set max possible Quantity
		val = *resource.NewQuantity(math.MaxInt64, current.Format)
	}

	if max != nil {
		val = util.MinQuantity(val, *max)
	}

	if min != nil {
		val = util.MaxQuantity(val, *min)
	}

	return val
}
