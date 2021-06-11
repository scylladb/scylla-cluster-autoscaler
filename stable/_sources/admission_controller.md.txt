# Admission Controller

Scylla Cluster Autoscaler's Admission Controller is essentially an admission webhook, which intercepts ScyllaCluster patch/update requests. If at a given time the object is being targeted by a ScyllaClusterAutoscaler in "Auto" mode, it checks whether the action does not change the attributes controlled by the autoscaler, or if has been performed by the Updater component by comparing its [Service Account](https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin) against Updater's Service Account Username. If it does change controlled attributes, or the author of the action is not the Updater component, it rejects the request with an appropriate error message. Therefore it prevents any other applications and the user from interrupting in an ongoing autoscaling process and thus protects its performance from any external disturbance.

## YAML
```yaml
spec:
  selector:
    matchLabels:
      control-plane: admission-controller
  replicas: 1
  template:
    metadata:
      labels:
        control-plane: admission-controller
    spec:
      serviceAccountName: admission-controller-service-account
      containers:
        - command:
            - /usr/bin/admission-controller
          args:
            - admission-controller
          image: admission-controller:latest
          imagePullPolicy: Always
          name: admission-controller
          resources:
            requests:
              cpu: 20m
              memory: 20Mi
      terminationGracePeriodSeconds: 10
  ```
