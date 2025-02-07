package controller

// The following labels are added to each nginx resource created by the control plane.
const (
	GatewayLabel      = "gateway.networking.k8s.io/gateway-name"
	AppNameLabel      = "app.kubernetes.io/name"
	AppInstanceLabel  = "app.kubernetes.io/instance"
	AppManagedByLabel = "app.kubernetes.io/managed-by"
)

// RestartedAnnotation is added to a Deployment or DaemonSet's PodSpec to trigger a rolling restart.
const RestartedAnnotation = "kubectl.kubernetes.io/restartedAt"
