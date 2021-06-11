# Updater

Updater is a component designed to apply the recommendations to the ScyllaClusters undergoing autoscaling. Similarly to Recommender, it observes the cluster in search of ScyllaClusterAutoscaler objects in "Auto" mode and periodically updates the targets' specifications with the provided recommendations. Additionally, it ensures that applying the changes is not going to disrupt the targets' state by following the update policies provided by the user.

## YAML
```yaml
spec:
  selector:
    matchLabels:
      control-plane: updater
  replicas: 1
  template:
    metadata:
      labels:
        control-plane: updater
    spec:
      serviceAccountName: updater-service-account
      containers:
        - command:
            - /usr/bin/updater
          args:
            - updater
            - --interval=120s
          image: updater:latest
          imagePullPolicy: Always
          name: updater
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

* `args`: flags for Updater
  * `--interval`: Updater main loop running interval.
