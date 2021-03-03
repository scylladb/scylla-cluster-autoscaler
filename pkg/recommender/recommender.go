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
	scas := &v1alpha1.ScyllaClusterAutoscalerList{}
	err := r.client.List(ctx, scas)
	if err != nil {
		return errors.Wrap(err, "fetch SCAs")
	}

	for idx, _ := range scas.Items {
		sca := &scas.Items[idx]
		targetRef := sca.Spec.TargetRef

		sc := &scyllav1.ScyllaCluster{}
		err = r.client.Get(ctx, client.ObjectKey{
			Namespace: targetRef.Namespace,
			Name:      targetRef.Name,
		}, sc)
		if err != nil {
			r.logger.Error(ctx, "fetch referenced ScyllaCluster", "error", err)
			continue
		}

		var dcRecommendations []v1alpha1.DataCenterRecommendations
		for _, dcScalingPolicy := range sca.Spec.ScalingPolicy.Datacenters {
			dc := sc.Spec.Datacenter
			if dcScalingPolicy.Name != dc.Name {
				r.logger.Error(ctx, "datacenter not found", "datacenter", dcScalingPolicy.Name)
				continue
			}

			var rackRecommendations []v1alpha1.RackRecommendations
			for _, rackScalingPolicy := range sca.Spec.ScalingPolicy.Datacenters[0].RackScalingPolicies {
				var rack scyllav1.RackSpec

				found := false
				for i := range dc.Racks {
					if dc.Racks[i].Name == rackScalingPolicy.Name {
						rack = dc.Racks[i]
						found = true
						break
					}
				}

				if !found {
					r.logger.Error(ctx, "rack not found", "rack", rackScalingPolicy.Name)
					continue
				}

				var priority int32 = math.MaxInt32
				recommendedRackMembers := v1alpha1.RecommendedRackMembers{Target: rack.Members}
				recommendedRackResources := v1alpha1.RecommendedRackResources{TargetCPU: *rack.Resources.Requests.Cpu()}
				for _, rule := range rackScalingPolicy.ScalingRules {
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

					if res {
						r.logger.Info(ctx, "Condition met. Cluster will be scaled...", "rule", rule.Name)
					} else {
						continue
					}

					if rule.ScalingMode == v1alpha1.ScalingModeHorizontal {
						// TODO check if value * factor overflows???
						calc := int32(rule.ScalingFactor * float64(rack.Members))

						if rackScalingPolicy.MemberPolicy != nil && rackScalingPolicy.MemberPolicy.MaxAllowed != nil {
							calc = util.MinInt32(calc, *rackScalingPolicy.MemberPolicy.MaxAllowed)
						}

						if rackScalingPolicy.MemberPolicy != nil && rackScalingPolicy.MemberPolicy.MinAllowed != nil {
							calc = util.MaxInt32(calc, *rackScalingPolicy.MemberPolicy.MinAllowed)
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

						if rackScalingPolicy.ResourcePolicy != nil && rackScalingPolicy.ResourcePolicy.MaxAllowedCpu != nil {
							calc = util.MinQuantity(calc, rackScalingPolicy.ResourcePolicy.MaxAllowedCpu)
						}

						if rackScalingPolicy.ResourcePolicy != nil && rackScalingPolicy.ResourcePolicy.MinAllowedCpu != nil {
							calc = util.MaxQuantity(calc, rackScalingPolicy.ResourcePolicy.MinAllowedCpu)
						}

						recommendedRackResources.TargetCPU = *calc
					}

					priority = rule.Priority
				}

				rackRecommendations = append(rackRecommendations, v1alpha1.RackRecommendations{Name: rack.Name, Members: &recommendedRackMembers, Resources: &recommendedRackResources})
				r.logger.Info(ctx, "recommendations", "cluster", sc.Name, "datacenter", dc.Name, "rackRecommendations", rackRecommendations)
			}

			dcRecommendations = append(dcRecommendations, v1alpha1.DataCenterRecommendations{Name: dc.Name, RackRecommendations: rackRecommendations})
		}

		sca.Status.Recommendations = &v1alpha1.ScyllaClusterRecommendations{DataCenterRecommendations: dcRecommendations}
		err = r.client.Status().Update(ctx, sca)
		if err != nil {
			r.logger.Error(ctx, "SCA status update", "error", err)
			continue
		}
	}

	return nil
}
