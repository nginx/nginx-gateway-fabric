package upstreamsettings

import (
	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
)

// Processor processes UpstreamSettingsPolicies.
type Processor struct{}

// UpstreamSettings contains settings from UpstreamSettingsPolicy.
type UpstreamSettings struct {
	// UseClusterIP indicates whether to route to the Service ClusterIP instead of Pod IPs.
	// A nil value means the policy did not set this field, allowing callers to fall back to
	// other configuration (e.g. the NginxProxy setting).
	UseClusterIP *bool
	// ZoneSize is the zone size setting.
	// A nil value means the policy did not set this field, allowing callers to fall back to
	// global configuration (e.g. the NginxProxy setting).
	ZoneSize *ngfAPI.Size
	// HealthCheck is the health check setting.
	HealthCheck *http.HealthCheck
	// LoadBalancingMethod is the load balancing method setting.
	LoadBalancingMethod string
	// HashMethodKey is the key to be used for hash-based load balancing methods.
	HashMethodKey string
	// KeepAlive contains the keepalive settings.
	KeepAlive http.UpstreamKeepAlive
}

// NewProcessor returns a new Processor.
func NewProcessor() Processor {
	return Processor{}
}

// Process processes policies into an UpstreamSettings object. The policies are already validated and are guaranteed
// to not contain overlapping settings. This method merges all fields in the policies into a single UpstreamSettings
// object.
func (g Processor) Process(pols []policies.Policy) UpstreamSettings {
	return processPolicies(pols)
}

// processPolicies merges a list of policies into a single UpstreamSettings configuration.
func processPolicies(pols []policies.Policy) UpstreamSettings {
	upstreamSettings := UpstreamSettings{}

	for _, pol := range pols {
		usp, ok := pol.(*ngfAPI.UpstreamSettingsPolicy)
		if !ok {
			continue
		}

		// we can assume that there will be no instance of two or more policies setting the same
		// field for the same service
		if usp.Spec.ZoneSize != nil {
			upstreamSettings.ZoneSize = usp.Spec.ZoneSize
		}

		if usp.Spec.KeepAlive != nil {
			if usp.Spec.KeepAlive.Connections != nil {
				upstreamSettings.KeepAlive.Connections = usp.Spec.KeepAlive.Connections
			}

			if usp.Spec.KeepAlive.Requests != nil {
				upstreamSettings.KeepAlive.Requests = *usp.Spec.KeepAlive.Requests
			}

			if usp.Spec.KeepAlive.Time != nil {
				upstreamSettings.KeepAlive.Time = string(*usp.Spec.KeepAlive.Time)
			}

			if usp.Spec.KeepAlive.Timeout != nil {
				upstreamSettings.KeepAlive.Timeout = string(*usp.Spec.KeepAlive.Timeout)
			}
		}

		if usp.Spec.HealthCheck != nil {
			processHealthCheck(usp.Spec.HealthCheck, &upstreamSettings)
		}

		if usp.Spec.LoadBalancingMethod != nil {
			upstreamSettings.LoadBalancingMethod = string(*usp.Spec.LoadBalancingMethod)
		}

		if usp.Spec.HashMethodKey != nil {
			upstreamSettings.HashMethodKey = string(*usp.Spec.HashMethodKey)
		}

		if usp.Spec.UseClusterIP != nil {
			upstreamSettings.UseClusterIP = usp.Spec.UseClusterIP
		}
	}

	return upstreamSettings
}

func processHealthCheck(hc *ngfAPI.HealthCheck, upstreamSettings *UpstreamSettings) {
	if upstreamSettings.HealthCheck == nil {
		upstreamSettings.HealthCheck = &http.HealthCheck{}
	}

	if hc.Passive != nil {
		processPassiveHealthCheck(hc.Passive, upstreamSettings)
	}

	if hc.Active != nil {
		processActiveHealthCheck(hc.Active, upstreamSettings)
	}
}

func processPassiveHealthCheck(
	passive *ngfAPI.PassiveHealthCheck,
	upstreamSettings *UpstreamSettings,
) {
	if upstreamSettings.HealthCheck.Passive == nil {
		upstreamSettings.HealthCheck.Passive = &http.PassiveHealthCheck{}
	}
	if passive.MaxFails != nil {
		upstreamSettings.HealthCheck.Passive.MaxFails = passive.MaxFails
	}
	if passive.FailTimeout != nil {
		upstreamSettings.HealthCheck.Passive.FailTimeout = string(*passive.FailTimeout)
	}
}

func processActiveHealthCheck(
	active *ngfAPI.ActiveHealthCheck,
	upstreamSettings *UpstreamSettings,
) {
	if upstreamSettings.HealthCheck.Active == nil {
		upstreamSettings.HealthCheck.Active = &http.ActiveHealthCheck{}
	}
	if active.Interval != nil {
		upstreamSettings.HealthCheck.Active.Interval = (*string)(active.Interval)
	}
	if active.Jitter != nil {
		upstreamSettings.HealthCheck.Active.Jitter = (*string)(active.Jitter)
	}
	if active.Fails != nil {
		upstreamSettings.HealthCheck.Active.Fails = active.Fails
	}
	if active.Passes != nil {
		upstreamSettings.HealthCheck.Active.Passes = active.Passes
	}
	if active.Path != nil {
		upstreamSettings.HealthCheck.Active.Path = active.Path
	}
	if active.Port != nil {
		upstreamSettings.HealthCheck.Active.Port = active.Port
	}
	if active.Match != nil {
		processHealthCheckMatch(active, upstreamSettings)
	}
	if active.GRPC != nil {
		processGRPCHealthCheck(active, upstreamSettings)
	}
	if active.Mandatory != nil {
		upstreamSettings.HealthCheck.Active.Mandatory = active.Mandatory
	}
	if active.Persistent != nil {
		upstreamSettings.HealthCheck.Active.Persistent = active.Persistent
	}
	if active.KeepAliveTime != nil {
		upstreamSettings.HealthCheck.Active.KeepAliveTime = (*string)(active.KeepAliveTime)
	}
	if active.Timeout != nil {
		processHealthCheckTimeout(active, upstreamSettings)
	}
	if active.Headers != nil {
		processHealthCheckHeaders(active, upstreamSettings)
	}
}

func processHealthCheckMatch(
	active *ngfAPI.ActiveHealthCheck,
	upstreamSettings *UpstreamSettings,
) {
	if upstreamSettings.HealthCheck.Active.Match == nil {
		upstreamSettings.HealthCheck.Active.Match = &http.Match{}
	}
	if active.Match.Status != nil {
		upstreamSettings.HealthCheck.Active.Match.Status = active.Match.Status
	}
}

func processGRPCHealthCheck(
	active *ngfAPI.ActiveHealthCheck,
	upstreamSettings *UpstreamSettings,
) {
	if upstreamSettings.HealthCheck.Active.GRPC == nil {
		upstreamSettings.HealthCheck.Active.GRPC = &http.GRPCHealthCheck{}
	}
	if active.GRPC.Service != nil {
		upstreamSettings.HealthCheck.Active.GRPC.Service = active.GRPC.Service
	}
	if active.GRPC.Status != nil {
		status := grpcStatusCode(*active.GRPC.Status)
		upstreamSettings.HealthCheck.Active.GRPC.Status = &status
	}
}

func grpcStatusCode(status ngfAPI.GRPCStatus) string {
	statusCodes := map[ngfAPI.GRPCStatus]string{
		ngfAPI.GRPCStatusCancelled:          "1",
		ngfAPI.GRPCStatusUnknown:            "2",
		ngfAPI.GRPCStatusInvalidArgument:    "3",
		ngfAPI.GRPCStatusDeadlineExceeded:   "4",
		ngfAPI.GRPCStatusNotFound:           "5",
		ngfAPI.GRPCStatusAlreadyExists:      "6",
		ngfAPI.GRPCStatusPermissionDenied:   "7",
		ngfAPI.GRPCStatusResourceExhausted:  "8",
		ngfAPI.GRPCStatusFailedPrecondition: "9",
		ngfAPI.GRPCStatusAborted:            "10",
		ngfAPI.GRPCStatusOutOfRange:         "11",
		ngfAPI.GRPCStatusUnimplemented:      "12",
		ngfAPI.GRPCStatusInternal:           "13",
		ngfAPI.GRPCStatusUnavailable:        "14",
		ngfAPI.GRPCStatusDataLoss:           "15",
		ngfAPI.GRPCStatusUnauthenticated:    "16",
	}

	if code, ok := statusCodes[status]; ok {
		return code
	}

	return string(status)
}

func processHealthCheckTimeout(
	active *ngfAPI.ActiveHealthCheck,
	upstreamSettings *UpstreamSettings,
) {
	if upstreamSettings.HealthCheck.Active.Timeout == nil {
		upstreamSettings.HealthCheck.Active.Timeout = &http.ProxyTimeout{}
	}
	if active.Timeout.Connect != nil {
		upstreamSettings.HealthCheck.Active.Timeout.Connect = (*string)(active.Timeout.Connect)
	}
	if active.Timeout.Read != nil {
		upstreamSettings.HealthCheck.Active.Timeout.Read = (*string)(active.Timeout.Read)
	}
	if active.Timeout.Send != nil {
		upstreamSettings.HealthCheck.Active.Timeout.Send = (*string)(active.Timeout.Send)
	}
}

func processHealthCheckHeaders(
	active *ngfAPI.ActiveHealthCheck,
	upstreamSettings *UpstreamSettings,
) {
	for _, header := range active.Headers {
		internalHeader := http.RequestHeader{
			Name:  string(header.Name),
			Value: header.Value,
		}

		upstreamSettings.HealthCheck.Active.Headers = append(
			upstreamSettings.HealthCheck.Active.Headers,
			internalHeader,
		)
	}
}
