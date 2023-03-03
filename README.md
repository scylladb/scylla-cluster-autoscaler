# [**DEPRECATED**] Scylla Cluster Autoscaler 

Scylla Cluster Autoscaler was an experimental project and is **no longer supported or maintained**.

---

# Original purpose 

[Scylla Cluster Autoscaler](https://github.com/scylladb/scylla-cluster-autoscaler) is an open source project which helps users of Scylla Open Source and Scylla Enterprise work with Scylla on Kubernetes (K8s).

The Scylla Cluster Autoscaler is an application capable of scaling Scylla database clusters in an automated manner, freeing the human operator from the task of updating the required specification manually. 
Using the elastic scale pattern, the autoscaler manages to scale the resources both vertically and horizontally.
Basic principle of SCA is that user defines a set of boolean queries on external, performance-related metrics and the actions to be invoked, were their evaluated values true. 
It can be also be run in mode in which it produces recommendations, but does not invoke provided scaling actions.
SCA is also equipped with a safety mechanism which prevents sources other than SCA itself scaling controlled resources of a managed cluster.

## Scylla Documentation

To see the full documentation visit Scylla Cluster Autoscaler's page in [Scylla Documentation](https://scylladb.github.io/scylla-cluster-autoscaler/stable/).

## Deploying Scylla Cluster Autoscaler

### Prerequisites

* A Kubernetes cluster (version >= 1.19)
* Controller-gen (version >= 0.4.1)

### Recommender, Updater, Admission Controller

Following command deploys all 3 components of the autoscaler in the configured Kubernetes cluster in ~/.kube/config:
```console
make deploy
```

Check if the components are up and running with:
```console
kubectl -n scylla-operator-autoscaler-system get pods
```
The output should be something like this:
```console
NAME                                                              READY   STATUS             RESTARTS   AGE
scylla-operator-autoscaler-admission-controller-7b4c7967ff4m9bs   1/1     Running            0          6m34s
scylla-operator-autoscaler-recommender-74bc4c995b-5q4kz           1/1     Running            0          6m34s
scylla-operator-autoscaler-updater-68898fd4c5-zpgcp               1/1     Running            0          6m34s
```

In case of running into problems check the logs of a particular component, for example:
```console
kubectl -n scylla-operator-autoscaler-system logs -f scylla-operator-autoscaler-recommender-74bc4c995b-5q4kz
```
and look at events:
```console
kubectl -n scylla-operator-autoscaler-system describe pod/scylla-operator-autoscaler-recommender-74bc4c995b-5q4kz
```

### Scylla Cluster Autoscaler
Then create CRD by applying the yaml file describing SCA.
```console
kubectl apply -f config/examples/generic/sca.yaml
```
Verify if it was created by:
```console
kubectl get scyllaclusterautoscalers
```
The output should be something like this:
```console
NAME         AGE
simple-sca   6m15s
```

You can look at the current description of the Scylla Cluster Autoscaler by entering:
```console
kubectl describe scyllaclusterautoscalers simple-sca
```
or edit the spec with a following command:
```console
kubectl edit scyllaclusterautoscalers simple-sca
```

### Clean Up

To clean up all resources associated with this walk-through, you can run the commands below.

```console
kubectl delete namespace scylla-operator-autoscaler-system
```

## Development
If you make some changes to the autoscaler and want to test it, you can do it locally by running:
```console
make build
```
One caveat is that admission controller won't be running properly this way as it is essentially a webhook.

Recommended way is to run it on a cluster with following commands:
```console
make images
make push-images
make deploy
```
previously swapping the IMAGE_REPO variable in the Makefile for your own repository.


## Contributing

If you want to contribute to the Scylla Cluster Autoscaler make sure the code is formatted properly by running:
```console
make verify
```
first and running:
```console
make update-gofmt
```
if asked to. Then refer to our [contributing guide](https://operator.docs.scylladb.com/stable/contributing.html) for Scylla Operator.

