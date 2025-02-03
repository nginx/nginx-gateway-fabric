package graph

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/gatewayclass"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

// GatewayClass represents the GatewayClass resource.
type GatewayClass struct {
	// Source is the source resource.
	Source *v1.GatewayClass
	// Conditions include Conditions for the GatewayClass.
	Conditions []conditions.Condition
	// Valid shows whether the GatewayClass is valid.
	Valid bool
}

// processedGatewayClasses holds the resources that belong to NGF.
type processedGatewayClasses struct {
	Winner  *v1.GatewayClass
	Ignored map[types.NamespacedName]*v1.GatewayClass
}

// processGatewayClasses returns the "Winner" GatewayClass, which is defined in
// the command-line argument and references this controller, and a list of "Ignored" GatewayClasses
// that reference this controller, but are not named in the command-line argument.
// Also returns a boolean that says whether or not the GatewayClass defined
// in the command-line argument exists, regardless of which controller it references.
func processGatewayClasses(
	gcs map[types.NamespacedName]*v1.GatewayClass,
	gcName string,
	controllerName string,
) (processedGatewayClasses, bool) {
	processedGwClasses := processedGatewayClasses{}

	var gcExists bool
	for _, gc := range gcs {
		if gc.Name == gcName {
			gcExists = true
			if string(gc.Spec.ControllerName) == controllerName {
				processedGwClasses.Winner = gc
			}
		} else if string(gc.Spec.ControllerName) == controllerName {
			if processedGwClasses.Ignored == nil {
				processedGwClasses.Ignored = make(map[types.NamespacedName]*v1.GatewayClass)
			}
			processedGwClasses.Ignored[client.ObjectKeyFromObject(gc)] = gc
		}
	}

	return processedGwClasses, gcExists
}

func buildGatewayClass(
	gc *v1.GatewayClass,
	npCfg *NginxProxy,
	crdVersions map[types.NamespacedName]*metav1.PartialObjectMetadata,
) *GatewayClass {
	if gc == nil {
		return nil
	}

	conds, valid := validateGatewayClass(gc, npCfg, crdVersions)

	return &GatewayClass{
		Source:     gc,
		Valid:      valid,
		Conditions: conds,
	}
}

func validateGatewayClass(
	gc *v1.GatewayClass,
	npCfg *NginxProxy,
	crdVersions map[types.NamespacedName]*metav1.PartialObjectMetadata,
) ([]conditions.Condition, bool) {
	var conds []conditions.Condition

	if gc.Spec.ParametersRef != nil {
		var err error
		path := field.NewPath("spec").Child("parametersRef")
		if _, ok := supportedParamKinds[string(gc.Spec.ParametersRef.Kind)]; !ok {
			err = field.NotSupported(path.Child("kind"), string(gc.Spec.ParametersRef.Kind), []string{kinds.NginxProxy})
		} else if npCfg == nil {
			err = field.NotFound(path.Child("name"), gc.Spec.ParametersRef.Name)
			conds = append(conds, staticConds.NewGatewayClassRefNotFound())
		} else if !npCfg.Valid {
			err = errors.New(npCfg.ErrMsgs.ToAggregate().Error())
		}

		if err != nil {
			conds = append(conds, staticConds.NewGatewayClassInvalidParameters(err.Error()))
		} else {
			conds = append(conds, staticConds.NewGatewayClassResolvedRefs())
		}
	}

	supportedVersionConds, versionsValid := gatewayclass.ValidateCRDVersions(crdVersions)

	return append(conds, supportedVersionConds...), versionsValid
}

var supportedParamKinds = map[string]struct{}{
	kinds.NginxProxy: {},
}
