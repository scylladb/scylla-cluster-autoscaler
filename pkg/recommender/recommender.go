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

	for idx, _ := range scas.Items {
		sca := &scas.Items[idx]
		targetRef := sca.Spec.TargetRef
		sc, err := r.fetchScyllaCluster(ctx, targetRef.Name, targetRef.Namespace)
		if err != nil {
			r.logger.Error(ctx, "fetch referenced ScyllaCluster", "error", err)
			continue
		}

		sca.Status.LastUpdated = metav1.NewTime(time.Now())
		sca.Status.Recommendations = r.getScyllaClusterRecommendations(ctx, sc, sca.Spec.ScalingPolicy)

		err = r.client.Status().Update(ctx, sca)
		if err != nil {
			r.logger.Error(ctx, "SCA status update", "error", err)
			continue
		}
	}

	return nil
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

func (r *recommender) getScyllaClusterRecommendations(ctx context.Context, sc *scyllav1.ScyllaCluster, scalingPolicy *v1alpha1.ScalingPolicy) *v1alpha1.ScyllaClusterRecommendations {
	var datacenterRecommendations []v1alpha1.DatacenterRecommendations
	for _, datacenterScalingPolicy := range scalingPolicy.Datacenters {
		datacenter := sc.Spec.Datacenter
		if datacenterScalingPolicy.Name != datacenter.Name {
			r.logger.Error(ctx, "datacenter not found", "datacenter", datacenterScalingPolicy.Name)
			continue
		}

		datacenterRecommendations = append(datacenterRecommendations, *r.getDatacenterRecommendations(ctx, &datacenter, &datacenterScalingPolicy))
	}

	return &v1alpha1.ScyllaClusterRecommendations{DatacenterRecommendations: datacenterRecommendations}
}

func (r *recommender) getDatacenterRecommendations(ctx context.Context, datacenter *scyllav1.DatacenterSpec, scalingPolicy *v1alpha1.DatacenterScalingPolicy) *v1alpha1.DatacenterRecommendations {
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
			r.logger.Error(ctx, "rack not found", "rack", rackScalingPolicy.Name)
			continue
		}

		rackRecommendations = append(rackRecommendations, *r.getRackRecommendations(ctx, &rack, &rackScalingPolicy))
	}

	return &v1alpha1.DatacenterRecommendations{Name: datacenter.Name, RackRecommendations: rackRecommendations}
}

func (r *recommender) getRackRecommendations(ctx context.Context, rack *scyllav1.RackSpec, scalingPolicy *v1alpha1.RackScalingPolicy) *v1alpha1.RackRecommendations {
	var err error
	var priority int32 = math.MaxInt32
	members := rack.Members
	resources := rack.Resources

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
			r.logger.Error(ctx, "fetch rack metrics", "rack", rack.Name, "error", err)
			continue
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

			members = calculateMembers(rack.Members, min, max, rule.ScalingFactor)
		} else {
			if rack.Resources.Requests == nil || rack.Resources.Requests.Cpu() == nil {
				r.logger.Error(ctx, "cpu requests undefined")
				continue
			}

			var min, max *resource.Quantity = nil, nil
			if scalingPolicy.ResourcePolicy != nil {
				min = scalingPolicy.ResourcePolicy.MinAllowedCpu
				max = scalingPolicy.ResourcePolicy.MaxAllowedCpu
			}
			resources.Requests[corev1.ResourceCPU] = calculateCPU(rack.Resources.Requests.Cpu(), min, max, rule.ScalingFactor)

			if rack.Resources.Limits != nil && rack.Resources.Limits.Cpu() != nil {
				if scalingPolicy.ResourcePolicy.RackControlledValues == v1alpha1.RackControlledValuesRequestsAndLimits {
					resources.Limits[corev1.ResourceCPU] = calculateCPU(rack.Resources.Limits.Cpu(), min, max, rule.ScalingFactor)
				} else {
					resources.Requests[corev1.ResourceCPU] = util.MinQuantity(resources.Requests[corev1.ResourceCPU], resources.Limits[corev1.ResourceCPU])
				}
			}
		}

		priority = rule.Priority
	}

	return &v1alpha1.RackRecommendations{Name: rack.Name, Members: &members, Resources: &resources}
}

func calculateMembers(current int32, min, max *int32, factor float64) int32 {
	val := int32(factor * float64(current))

	if max != nil {
		val = util.MinInt32(val, *max)
	}

	if min != nil {
		val = util.MaxInt32(val, *min)
	}

	return val
}

func calculateCPU(current, min, max *resource.Quantity, factor float64) resource.Quantity {
	var val resource.Quantity

	// TODO keep original scale???
	// TODO check if this makes any sense
	if current.Value() > (math.MaxInt64 / 1000) {
		val = *resource.NewQuantity(int64(factor*float64(current.Value())), current.Format)
	} else {
		val = *resource.NewMilliQuantity(int64(factor*float64(current.ScaledValue(resource.Milli))), current.Format)
	}

	if max != nil {
		val = util.MinQuantity(val, *max)
	}

	if min != nil {
		val = util.MaxQuantity(val, *min)
	}

	return val
}
