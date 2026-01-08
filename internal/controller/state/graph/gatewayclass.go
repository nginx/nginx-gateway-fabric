package graph

import (
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/pkg/consts"
	"sigs.k8s.io/gateway-api/pkg/features"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

var gatewayCRDs = map[string]apiVersion{
	"gatewayclasses.gateway.networking.k8s.io":     {},
	"gateways.gateway.networking.k8s.io":           {},
	"httproutes.gateway.networking.k8s.io":         {},
	"referencegrants.gateway.networking.k8s.io":    {},
	"backendtlspolicies.gateway.networking.k8s.io": {},
	"grpcroutes.gateway.networking.k8s.io":         {},
	"tlsroutes.gateway.networking.k8s.io":          {},
}

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
	// ExperimentalSupported indicates whether experimental features are supported.
	ExperimentalSupported bool
	// BestEffort indicates whether the Gateway API CRD versions are not supported,
	// but we are attempting to generate configuration.
	BestEffort bool
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
	experimentalEnabled bool,
) *GatewayClass {
	if gc == nil {
		return nil
	}

	var np *NginxProxy
	if gc.Spec.ParametersRef != nil {
		np = getNginxProxyForGatewayClass(*gc.Spec.ParametersRef, nps)
	}

	conds, valid, crdExperimental, bestEffort := validateGatewayClass(gc, np, crdVersions)

	// Experimental features are supported only if both the config flag AND CRD channel are experimental
	experimental := experimentalEnabled && crdExperimental

	return &GatewayClass{
		Source:                gc,
		NginxProxy:            np,
		Valid:                 valid,
		Conditions:            conds,
		ExperimentalSupported: experimental,
		BestEffort:            bestEffort,
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
		msg := helpers.CapitalizeString(errs.ToAggregate().Error())
		return []conditions.Condition{
			conditions.NewGatewayClassRefInvalid(msg),
			conditions.NewGatewayClassInvalidParameters(msg),
		}
	}

	return nil
}

func validateGatewayClass(
	gc *v1.GatewayClass,
	npCfg *NginxProxy,
	crdVersions map[types.NamespacedName]*metav1.PartialObjectMetadata,
) ([]conditions.Condition, bool, bool, bool) {
	var conds []conditions.Condition

	supportedVersionConds, versionsValid, experimental, bestEffort := validateCRDVersions(crdVersions)
	conds = append(conds, supportedVersionConds...)

	if gc.Spec.ParametersRef == nil {
		return conds, versionsValid, experimental, bestEffort
	}

	path := field.NewPath("spec").Child("parametersRef")
	refConds := validateGatewayClassParametersRef(path, *gc.Spec.ParametersRef)

	// return early since parametersRef isn't valid
	if len(refConds) > 0 {
		conds = append(conds, refConds...)
		return conds, versionsValid, experimental, bestEffort
	}

	if npCfg == nil {
		conds = append(
			conds,
			conditions.NewGatewayClassRefNotFound(),
			conditions.NewGatewayClassInvalidParameters(
				field.NotFound(path.Child("name"), gc.Spec.ParametersRef.Name).Error(),
			),
		)
		return conds, versionsValid, experimental, bestEffort
	}

	if !npCfg.Valid {
		msg := helpers.CapitalizeString(npCfg.ErrMsgs.ToAggregate().Error())
		conds = append(
			conds,
			conditions.NewGatewayClassRefInvalid(msg),
			conditions.NewGatewayClassInvalidParameters(msg),
		)
		return conds, versionsValid, experimental, bestEffort
	}

	return append(conds, conditions.NewGatewayClassResolvedRefs()), versionsValid, experimental, bestEffort
}

var supportedParamKinds = map[string]struct{}{
	kinds.NginxProxy: {},
}

type apiVersion struct {
	major string
	minor string
}

func validateCRDVersions(
	crdMetadata map[types.NamespacedName]*metav1.PartialObjectMetadata,
) (conds []conditions.Condition, valid bool, experimental bool, bestEffort bool) {
	installedAPIVersions, channels := getBundleVersions(crdMetadata)
	supportedAPIVersion := parseVersionString(consts.BundleVersion)

	var unsupported bool

	for _, version := range installedAPIVersions {
		if version.major != supportedAPIVersion.major {
			unsupported = true
		} else if version.minor != supportedAPIVersion.minor {
			bestEffort = true
		}
	}

	// Check if any CRD is using experimental channel
	if slices.Contains(channels, features.FeatureChannelExperimental) {
		experimental = true
	}

	if unsupported {
		return conditions.NewGatewayClassUnsupportedVersion(consts.BundleVersion), false, experimental, false
	}

	if bestEffort {
		return conditions.NewGatewayClassSupportedVersionBestEffort(consts.BundleVersion), true, experimental, bestEffort
	}

	return nil, true, experimental, false
}

func parseVersionString(version string) apiVersion {
	versionBits := strings.Split(version, ".")
	if len(versionBits) < 3 {
		return apiVersion{}
	}

	major, _ := strings.CutPrefix(versionBits[0], "v")

	// Handle pre-release versions like "v1.4.0-rc.2" by stripping anything after the minor version
	minor := versionBits[1]
	// For pre-release versions like "0-rc", we just want "0"
	if idx := strings.Index(versionBits[2], "-"); idx != -1 {
		// This is a pre-release version, minor is in versionBits[1]
		minor = versionBits[1]
	}

	return apiVersion{
		major: major,
		minor: minor,
	}
}

func getBundleVersions(
	crdMetadata map[types.NamespacedName]*metav1.PartialObjectMetadata,
) ([]apiVersion, []features.FeatureChannel) {
	versions := make([]apiVersion, 0, len(gatewayCRDs))
	channels := make([]features.FeatureChannel, 0, len(gatewayCRDs))

	for nsname, md := range crdMetadata {
		if _, ok := gatewayCRDs[nsname.Name]; ok {
			bundleVersion := md.Annotations[consts.BundleVersionAnnotation]
			versions = append(versions, parseVersionString(bundleVersion))

			// Default to standard channel if annotation is missing
			ch := md.Annotations[consts.ChannelAnnotation]
			if ch == "" {
				ch = string(features.FeatureChannelStandard)
			}
			channels = append(channels, features.FeatureChannel(ch))
		}
	}

	return versions, channels
}
