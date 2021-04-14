# Recommender

Recommender, Autoscaler's most vital component, connects with an external monitoring service and, using the user-defined queries, estimates the desired state of the scaling target.
Its primary concern is to estimate the ScyllaClusters' recommended resources. During its normal routine, the module examines the cluster for any existing SCA objects. Its goal then, for every given SCA, is to perform a set of queries to the monitoring system according to the `rules` provided by the user in the SCA CRD. Depending on the queries' results, it then computes the recommended specification and saves it in the SCA's status.

## YAML
```yaml
spec:
  selector:
    matchLabels:
      control-plane: recommender
  replicas: 1
  template:
    metadata:
      labels:
        control-plane: recommender
    spec:
      serviceAccountName: recommender-service-account
      containers:
        - command:
            - /usr/bin/recommender
          args:
            - recommender
            - --interval=10s
            - --metrics-selector-set=app=kube-prometheus-stack-prometheus
            - --metrics-default-step=60s
          image: recommender:latest
          imagePullPolicy: Always
          name: recommender
          resources:
            limits:
              cpu: 30m
              memory: 30Mi
            requests:
              cpu: 20m
              memory: 20Mi
      terminationGracePeriodSeconds: 10
```

## Elements of main interest to user:

* `args`: flags for Recommender
  * `--interval`: Recommender main loop running interval.
  * `--metrics-selector-set`: key=value label selector to used to identify desired monitoring service
  * `--metrics-default-step`: metrics ranged queries' default step
