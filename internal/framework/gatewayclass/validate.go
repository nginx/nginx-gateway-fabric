package gatewayclass

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
)

const (
	// BundleVersionAnnotation is the annotation on Gateway API CRDs that contains the installed version.
	BundleVersionAnnotation = "gateway.networking.k8s.io/bundle-version"
	// SupportedVersion is the supported version of the Gateway API CRDs.
	SupportedVersion = "v1.2.1"
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

type apiVersion struct {
	major string
	minor string
}

func ValidateCRDVersions(
	crdMetadata map[types.NamespacedName]*metav1.PartialObjectMetadata,
) (conds []conditions.Condition, valid bool) {
	installedAPIVersions := getBundleVersions(crdMetadata)
	supportedAPIVersion := parseVersionString(SupportedVersion)

	var unsupported, bestEffort bool

	for _, version := range installedAPIVersions {
		if version.major != supportedAPIVersion.major {
			unsupported = true
		} else if version.minor != supportedAPIVersion.minor {
			bestEffort = true
		}
	}

	if unsupported {
		return conditions.NewGatewayClassUnsupportedVersion(SupportedVersion), false
	}

	if bestEffort {
		return conditions.NewGatewayClassSupportedVersionBestEffort(SupportedVersion), true
	}

	return nil, true
}

func parseVersionString(version string) apiVersion {
	versionBits := strings.Split(version, ".")
	if len(versionBits) != 3 {
		return apiVersion{}
	}

	major, _ := strings.CutPrefix(versionBits[0], "v")

	return apiVersion{
		major: major,
		minor: versionBits[1],
	}
}

func getBundleVersions(crdMetadata map[types.NamespacedName]*metav1.PartialObjectMetadata) []apiVersion {
	versions := make([]apiVersion, 0, len(gatewayCRDs))

	for nsname, md := range crdMetadata {
		if _, ok := gatewayCRDs[nsname.Name]; ok {
			bundleVersion := md.Annotations[BundleVersionAnnotation]
			versions = append(versions, parseVersionString(bundleVersion))
		}
	}

	return versions
}
