package static

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/record"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
)

// updateControlPlane updates the control plane configuration with the given user spec.
// If any fields are not set within the user spec, the default configuration values are used.
func updateControlPlane(
	cfg *ngfAPI.NginxGateway,
	logger logr.Logger,
	eventRecorder record.EventRecorder,
	configNSName types.NamespacedName,
	logLevelSetter logLevelSetter,
) error {
	// build up default configuration
	controlConfig := ngfAPI.NginxGatewaySpec{
		Logging: &ngfAPI.Logging{
			Level: helpers.GetPointer(ngfAPI.ControllerLogLevelInfo),
		},
	}

	// by marshaling the user config and then unmarshaling on top of the default config,
	// we ensure that any unset user values are set with the default values
	if cfg != nil {
		cfgBytes, err := json.Marshal(cfg.Spec)
		if err != nil {
			return fmt.Errorf("error marshaling control config: %w", err)
		}

		if err := json.Unmarshal(cfgBytes, &controlConfig); err != nil {
			return fmt.Errorf("error unmarshaling control config: %w", err)
		}
	} else {
		msg := "NginxGateway configuration was deleted; using defaults"
		logger.Info(msg)
		eventRecorder.Event(
			&ngfAPI.NginxGateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: configNSName.Namespace,
					Name:      configNSName.Name,
				},
			},
			apiv1.EventTypeWarning,
			"ResourceDeleted",
			msg,
		)
	}

	level := *controlConfig.Logging.Level

	if err := validateLogLevel(level); err != nil {
		return err
	}

	if err := logLevelSetter.SetLevel(string(level)); err != nil {
		return field.Invalid(
			field.NewPath("logging.level"),
			level,
			err.Error(),
		)
	}

	return nil
}

func validateLogLevel(level ngfAPI.ControllerLogLevel) error {
	switch level {
	case ngfAPI.ControllerLogLevelInfo, ngfAPI.ControllerLogLevelDebug, ngfAPI.ControllerLogLevelError:
	default:
		return field.NotSupported(
			field.NewPath("logging.level"),
			level,
			[]string{
				string(ngfAPI.ControllerLogLevelInfo),
				string(ngfAPI.ControllerLogLevelDebug),
				string(ngfAPI.ControllerLogLevelError),
			})
	}

	return nil
}
