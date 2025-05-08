package graph

import (
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
	// NginxProxy is the NginxProxy resource referenced by this GatewayClass.
	NginxProxy *NginxProxy
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
// Also returns a boolean that says whether the GatewayClass defined
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
	nps map[types.NamespacedName]*NginxProxy,
	crdVersions map[types.NamespacedName]*metav1.PartialObjectMetadata,
) *GatewayClass {
	if gc == nil {
		return nil
	}

	var np *NginxProxy
	if gc.Spec.ParametersRef != nil {
		np = getNginxProxyForGatewayClass(*gc.Spec.ParametersRef, nps)
	}

	conds, valid := validateGatewayClass(gc, np, crdVersions)

	return &GatewayClass{
		Source:     gc,
		NginxProxy: np,
		Valid:      valid,
		Conditions: conds,
	}
}

func getNginxProxyForGatewayClass(
	ref v1.ParametersReference,
	nps map[types.NamespacedName]*NginxProxy,
) *NginxProxy {
	if ref.Namespace == nil {
		return nil
	}

	npName := types.NamespacedName{Name: ref.Name, Namespace: string(*ref.Namespace)}

	return nps[npName]
}

func validateGatewayClassParametersRef(path *field.Path, ref v1.ParametersReference) []conditions.Condition {
	var errs field.ErrorList

	if _, ok := supportedParamKinds[string(ref.Kind)]; !ok {
		errs = append(
			errs,
			field.NotSupported(path.Child("kind"), string(ref.Kind), []string{kinds.NginxProxy}),
		)
	}

	if ref.Namespace == nil {
		errs = append(errs, field.Required(path.Child("namespace"), "ParametersRef must specify Namespace"))
	}

	if len(errs) > 0 {
		msg := errs.ToAggregate().Error()
		return []conditions.Condition{
			staticConds.NewGatewayClassRefInvalid(msg),
			staticConds.NewGatewayClassInvalidParameters(msg),
		}
	}

	return nil
}

func validateGatewayClass(
	gc *v1.GatewayClass,
	npCfg *NginxProxy,
	crdVersions map[types.NamespacedName]*metav1.PartialObjectMetadata,
) ([]conditions.Condition, bool) {
	var conds []conditions.Condition

	supportedVersionConds, versionsValid := gatewayclass.ValidateCRDVersions(crdVersions)
	conds = append(conds, supportedVersionConds...)

	if gc.Spec.ParametersRef == nil {
		return conds, versionsValid
	}

	path := field.NewPath("spec").Child("parametersRef")
	refConds := validateGatewayClassParametersRef(path, *gc.Spec.ParametersRef)

	// return early since parametersRef isn't valid
	if len(refConds) > 0 {
		conds = append(conds, refConds...)
		return conds, versionsValid
	}

	if npCfg == nil {
		conds = append(
			conds,
			staticConds.NewGatewayClassRefNotFound(),
			staticConds.NewGatewayClassInvalidParameters(
				field.NotFound(path.Child("name"), gc.Spec.ParametersRef.Name).Error(),
			),
		)
		return conds, versionsValid
	}

	if !npCfg.Valid {
		msg := npCfg.ErrMsgs.ToAggregate().Error()
		conds = append(
			conds,
			staticConds.NewGatewayClassRefInvalid(msg),
			staticConds.NewGatewayClassInvalidParameters(msg),
		)
		return conds, versionsValid
	}

	return append(conds, staticConds.NewGatewayClassResolvedRefs()), versionsValid
}

var supportedParamKinds = map[string]struct{}{
	kinds.NginxProxy: {},
}
