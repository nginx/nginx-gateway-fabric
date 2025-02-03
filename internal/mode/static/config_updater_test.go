package static

import (
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/staticfakes"
)

func TestUpdateControlPlane(t *testing.T) {
	t.Parallel()
	debugLogCfg := &ngfAPI.NginxGateway{
		Spec: ngfAPI.NginxGatewaySpec{
			Logging: &ngfAPI.Logging{
				Level: helpers.GetPointer(ngfAPI.ControllerLogLevelDebug),
			},
		},
	}

	invalidLevelConfig := &ngfAPI.NginxGateway{
		Spec: ngfAPI.NginxGatewaySpec{
			Logging: &ngfAPI.Logging{
				Level: helpers.GetPointer[ngfAPI.ControllerLogLevel]("invalid"),
			},
		},
	}

	logger := zap.New()
	nsname := types.NamespacedName{Namespace: "test", Name: "test"}

	tests := []struct {
		setLevelErr          error
		nginxGateway         *ngfAPI.NginxGateway
		name                 string
		expErrString         string
		expSetLevelCallCount int
		expEvent             bool
	}{
		{
			name:                 "change log level",
			nginxGateway:         debugLogCfg,
			expSetLevelCallCount: 1,
		},
		{
			name:                 "invalid log level",
			nginxGateway:         invalidLevelConfig,
			expErrString:         `Unsupported value: "invalid"`,
			expSetLevelCallCount: 0,
		},
		{
			name:                 "nil NginxGateway",
			nginxGateway:         nil,
			expEvent:             true,
			expSetLevelCallCount: 1,
		},
		{
			name:                 "set log level fails",
			nginxGateway:         debugLogCfg,
			setLevelErr:          errors.New("set level failed"),
			expErrString:         "set level failed",
			expSetLevelCallCount: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fakeLogSetter := &staticfakes.FakeLogLevelSetter{
				SetLevelStub: func(_ string) error {
					return test.setLevelErr
				},
			}

			fakeEventRecorder := record.NewFakeRecorder(1)

			err := updateControlPlane(test.nginxGateway, logger, fakeEventRecorder, nsname, fakeLogSetter)

			if test.expErrString != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.expErrString))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if test.expEvent {
				g.Expect(fakeEventRecorder.Events).To(HaveLen(1))
				event := <-fakeEventRecorder.Events
				g.Expect(event).To(ContainSubstring("ResourceDeleted"))
			} else {
				g.Expect(fakeEventRecorder.Events).To(BeEmpty())
			}

			g.Expect(fakeLogSetter.SetLevelCallCount()).To(Equal(test.expSetLevelCallCount))
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	t.Parallel()
	validLevels := []ngfAPI.ControllerLogLevel{
		ngfAPI.ControllerLogLevelError,
		ngfAPI.ControllerLogLevelInfo,
		ngfAPI.ControllerLogLevelDebug,
	}

	invalidLevels := []ngfAPI.ControllerLogLevel{
		ngfAPI.ControllerLogLevel("invalid"),
		ngfAPI.ControllerLogLevel("high"),
		ngfAPI.ControllerLogLevel("warn"),
	}

	for _, level := range validLevels {
		t.Run(fmt.Sprintf("valid level %q", level), func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(validateLogLevel(level)).To(Succeed())
		})
	}

	for _, level := range invalidLevels {
		t.Run(fmt.Sprintf("invalid level %q", level), func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(validateLogLevel(level)).ToNot(Succeed())
		})
	}
}
