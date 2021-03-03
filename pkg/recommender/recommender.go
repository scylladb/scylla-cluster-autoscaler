package recommender

import (
	"context"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/util"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

func New(ctx context.Context, c client.Client, logger log.Logger, metricsSelector map[string]string) (Recommender, error) {
	p, err := metrics.NewPrometheusProvider(ctx, c, logger, metricsSelector)
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

	recommendedRackMembers := v1alpha1.RecommendedRackMembers{Target: rack.Members}
	recommendedRackResources := v1alpha1.RecommendedRackResources{TargetCPU: *rack.Resources.Requests.Cpu()}
	for _, rule := range scalingPolicy.ScalingRules {
		if rule.Priority >= priority {
			// TODO change priorities to a different system???
			continue // TODO solve conflicting priorities, i.e. two rules with equal priorities
		}

		var res bool
		if rule.For != nil {
			duration, err := time.ParseDuration(*rule.For)
			if err != nil {
				r.logger.Error(ctx, "invalid duration")
				continue
			}
			res, err = r.metricsProvider.RangedQuery(ctx, rule.Expression, duration)
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
			// TODO check if value * factor overflows???
			calc := int32(rule.ScalingFactor * float64(rack.Members))

			if scalingPolicy.MemberPolicy != nil && scalingPolicy.MemberPolicy.MaxAllowed != nil {
				calc = util.MinInt32(calc, *scalingPolicy.MemberPolicy.MaxAllowed)
			}

			if scalingPolicy.MemberPolicy != nil && scalingPolicy.MemberPolicy.MinAllowed != nil {
				calc = util.MaxInt32(calc, *scalingPolicy.MemberPolicy.MinAllowed)
			}

			recommendedRackMembers.Target = calc
		}

		// TODO Assertion: limits == requests???
		if rule.ScalingMode == v1alpha1.ScalingModeVertical {
			// TODO check if requests/limits exist???
			// TODO keep original scale???
			// TODO check if value * factor overflows???

			var calc *resource.Quantity
			if rack.Resources.Requests.Cpu().Value() > (math.MaxInt64 / 1000) {
				calc = resource.NewQuantity(int64(rule.ScalingFactor*float64(rack.Resources.Requests.Cpu().Value())), rack.Resources.Requests.Cpu().Format)
			} else {
				calc = resource.NewMilliQuantity(int64(rule.ScalingFactor*float64(rack.Resources.Requests.Cpu().ScaledValue(resource.Milli))), rack.Resources.Requests.Cpu().Format)
			}

			if scalingPolicy.ResourcePolicy != nil && scalingPolicy.ResourcePolicy.MaxAllowedCpu != nil {
				calc = util.MinQuantity(calc, scalingPolicy.ResourcePolicy.MaxAllowedCpu)
			}

			if scalingPolicy.ResourcePolicy != nil && scalingPolicy.ResourcePolicy.MinAllowedCpu != nil {
				calc = util.MaxQuantity(calc, scalingPolicy.ResourcePolicy.MinAllowedCpu)
			}

			recommendedRackResources.TargetCPU = *calc
		}

		priority = rule.Priority
	}

	return &v1alpha1.RackRecommendations{Name: rack.Name, Members: &recommendedRackMembers, Resources: &recommendedRackResources}
}
