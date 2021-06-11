# Scylla Cluster Autoscaler CRD

Scylla clusters can be created and configuring using the 'scyllaclusterautoscalers.scylla.scylladb.com' custom resource definition (CRD).

Please refer to the the [user guide walk-through](generic.md) for deployment instructions.
This page will explain all the available configuration options on the Scylla Cluster Autoscaler CRD.

## Sample

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaClusterAutoscaler
metadata:
  name: example-sca
spec:
  targetRef:
    namespace: scylla
    name: example-cluster
  updatePolicy:
    updateMode: Auto
  scalingPolicy:
    datacenters:
    - name: us-east-1
      racks:
      - name: us-east-1a
        rules:
        - name: cpu utilization horizontal up
          priority: 1
          expression: 'avg(scylla_reactor_utilization{scylla_cluster="example-cluster", scylla_datacenter="us-east-1", scylla_rack="us-east-1a"}) > bool 70'
          mode: Horizontal
          for: 10m
          step: 30s
          factor: 2
        - name: cpu utilization horizontal down
          priority: 1
          expression: 'avg(scylla_reactor_utilization{scylla_cluster="example-cluster", scylla_datacenter="us-east-1", scylla_rack="us-east-1a"}) < bool 10'
          mode: Horizontal
          for: 10m
          step: 30s
          factor: 0.5
        memberPolicy:
          minAllowed: 1
          maxAllowed: 5
        resourcePolicy:
          minAllowedCpu: 1
          maxAllowedCpu: 2
          controlledValues: Requests
status:
  lastApplied: 2021-04-14T11:54:17Z
  lastUpdated: 2021-04-14T16:43:22Z
  updateStatus: Ok
  recommendations:
    datacenterRecommendations:
      - name: us-east-1
        rackRecommendations:
          - name: us-east-1a
            members: 2
            resources:
              limits:
                cpu:     2
                memory:  30Gi
              requests:
                cpu:     2
                memory:  30Gi
```

## Autoscaler Settings

* `targetRef`: description if the ScyllaCluster object the autoscaling configuration is referring to. Comprising of `name` and `namespace`.
  * `namespace`: String. Namespace of ScyllaCluster
  * `name`: String. Name of ScyllaCluster

* `updatePolicy`: Optional field. Rules and limitations of how the target is meant to be updated.
  * `updateMode`: Enum, optional field. Can be set to either "Off" or "Auto" (default "Auto"). Recommendations are being provided and saved in either of these cases, however, they are only ever applied in the latter.
  * `recommendationExpirationTime`: [Duration](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration), optional field. How long the recommendations stay valid.
  * `updateCooldown`: [Duration](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration), optional field. Length of a period after updating ScyllaCluster, during which no other recommendations should be applied.

* `scalingPolicy`: Optional field. Rules and limitations of how specific datacenters and rack (identified by `name`) are meant to be scaled.
  * `rules`: descriptions of boolean queries (currently [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics) format is supported) and the actions to be invoked, were their evaluated values true. A simple query is only tested at the time of evaluation. A ranged query, on the other hand, is tested against a specified time range with a predetermined frequency. It only evaluates to true if the condition has been met at all points in the time series. A single rule is composed of the following:
    * `name`: String. Unique name of the rule.
    * `priority`: int32. Importance of a rule (minimum value is 0). One with the lowest priority is chosen over the others. For triggered rules with equal priority, their top to bottom order decides.
    * `expression`: String. Boolean query to the monitoring service.
    * `mode`: Enum. Can be set to either "Horizotal" or "Vertical" values which determine whether the target is to be scaled horizontally, by changing the number of Members, or vertically, by changing the amount of resources available for its operation.
    * `for`: [Duration](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration), optional field. If set, describes the duration of a ranged query. Expression must be satisfied at all points in the time series for this long in order to initiate scaling action.
    * `step`: [Duration](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration), optional field. Minimal time period between subsequent points in the time series. Effectively describes the frequency with which the expression will be queried. Only applies to a ranged query. 
    * `factor`: float64. Factor by which the scaled value will be multiplied.

* `memberPolicy`: Optional field. Limitations on scaling Rack's members. Safety mechanism to avoid scaling infinitely.
  * `minAllowed`: int32, optional field. Minimum number of Rack's members. SCA won't scale members below this number.
  * `maxAllowed`: int32, optional field. Maximum number of Rack's members. SCA won't scale members above this number.

* `resourcePolicy`: Optional field. Policy on scaling Rack's resources. 
  * `minAllowedCpu`: [Quantity](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/resource#Quantity), optional field. Minimum Rack's CPU resource quantity. SCA won't scale CPU resource below this quantity.
  * `maxAllowedCpu`: [Quantity](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/resource#Quantity), optional field. Maximum Rack's CPU resource quantity. SCA won't scale CPU resource above this quantity.
  * `controlledValues`: Enum, optional field. Can be set to either "Requests" or "RequestsAndLimits" (default "RequestsAndLimits"). Which resource values should be scaled.

## Autoscaler status
* `lastApplied`: [Time](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Time), optional field. Timestamp of last applied recommendations.
* `lastUpdated`: [Time](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Time), optional field. Timestamp of last saved recommendations.
* `updateStatus`: Enum, optional field. Is set to either "Ok", or "TargetFetchFail", or "TargetNotReady", or "RecommendationsFail". Values suggest that recommendations were prepared successfully, that the target ScyllaCluster could not be fetched, that the target was reachable but unstable, or that preparing recommendations resulted in an error, respectively.
* `recommendations`: Optional field. Recommendations for specific datacenters and racks (identified by `name`).
  * `name`: String. Name of the rack, recommendation is refering to.
  * `members`: int32, optional field. Recommended number of members for the Rack
  * `resources`: [ResourceRequirements](https://pkg.go.dev/k8s.io/api/core/v1#ResourceRequirements), optional field. Recommended resource quantity for the Rack
