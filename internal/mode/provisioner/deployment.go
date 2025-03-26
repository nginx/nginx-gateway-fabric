package provisioner

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// prepareDeployment prepares a new the static mode Deployment based on the YAML manifest.
// It will use the specified id to set unique parts of the deployment, so it must be unique among all Deployments for
// Gateways.
// It will configure the Deployment to use the Gateway with the given NamespacedName.
func prepareDeployment(depYAML []byte, id string, gwNsName types.NamespacedName) (*v1.Deployment, error) {
	dep := &v1.Deployment{}
	if err := yaml.Unmarshal(depYAML, dep); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment: %w", err)
	}

	dep.Name = id
	dep.Spec.Selector.MatchLabels["app"] = id
	dep.Spec.Template.Labels["app"] = id

	finalArgs := []string{
		"--gateway=" + gwNsName.String(),
		"--update-gatewayclass-status=false",
	}

	for _, arg := range dep.Spec.Template.Spec.Containers[0].Args {
		if strings.Contains(arg, "leader-election-lock-name") {
			lockNameArg := "--leader-election-lock-name=" + gwNsName.Name
			finalArgs = append(finalArgs, lockNameArg)
		} else {
			finalArgs = append(finalArgs, arg)
		}
	}

	dep.Spec.Template.Spec.Containers[0].Args = finalArgs

	return dep, nil
}
