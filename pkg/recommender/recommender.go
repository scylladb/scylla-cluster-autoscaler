package recommender

import (
	"context"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type (
	Recommender interface {
		RunOnce(ctx context.Context) error
	}

	recommender struct {
		client          client.Client
		logger          log.Logger
		metricsProvider metrics.Provider
	}
)

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

		ruleGroups := sca.Spec.ScalingRuleGroups

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

				group, found := ruleGroups[rackScalingPolicy.ScalingRuleGroup]
				if !found {
					r.logger.Error(ctx, "rule group not found")
					continue
				}

				recommendedMembers := rack.Members
				for _, rule := range group.Rules {
					labelsMap := rule.Labels
					labelsMap["scylla_cluster"] = sc.Name
					labelsMap["scylla_datacenter"] = dc.Name
					labelsMap["scylla_rack"] = rackScalingPolicy.Name

					res, err := r.metricsProvider.FetchMetric(ctx, rule.Metric, labelsMap)
					if err != nil {
						r.logger.Error(ctx, "fetch rack metrics", "rack", rack.Name, "err", err)
						continue
					}

					if res < rule.Threshold {
						continue
					}

					newMembers := rack.Members + rule.Instances
					if newMembers > recommendedMembers {
						recommendedMembers = newMembers
					}
				}

				rackRecommendations = append(rackRecommendations, v1alpha1.RackRecommendations{Name: rack.Name, Members: &v1alpha1.RecommendedRackMembers{Target: recommendedMembers}})
				r.logger.Info(ctx, "recommendations", "sca", sca.Name, "datacenter", dc.Name, "rackRecommendations", rackRecommendations)
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
