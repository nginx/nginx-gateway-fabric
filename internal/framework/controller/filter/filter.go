package filter

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
)

// CreateSingleResourceFilter creates a filter function that filters out all resources except the one
// with the given namespaced name.
func CreateSingleResourceFilter(targetNsName types.NamespacedName) controller.NamespacedNameFilterFunc {
	return func(nsname types.NamespacedName) (shouldProcess bool, msg string) {
		if nsname != targetNsName {
			msg := fmt.Sprintf(
				"Resource is ignored because this controller only supports a single resource %s/%s of that type",
				targetNsName.Namespace,
				targetNsName.Name,
			)
			return false, msg
		}
		return true, ""
	}
}
