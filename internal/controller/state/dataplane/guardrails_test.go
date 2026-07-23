package dataplane

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestConvertDurationToMS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in     *ngfAPIv1alpha1.Duration
		name   string
		expMS  int64
		expNil bool
	}{
		{name: "nil duration", in: nil, expNil: true},
		{name: "empty string", in: helpers.GetPointer[ngfAPIv1alpha1.Duration](""), expNil: true},
		{name: "seconds unit", in: helpers.GetPointer[ngfAPIv1alpha1.Duration]("30s"), expMS: 30000},
		{name: "milliseconds unit", in: helpers.GetPointer[ngfAPIv1alpha1.Duration]("500ms"), expMS: 500},
		{name: "minutes unit", in: helpers.GetPointer[ngfAPIv1alpha1.Duration]("2m"), expMS: 120000},
		{name: "bare number is seconds", in: helpers.GetPointer[ngfAPIv1alpha1.Duration]("5"), expMS: 5000},
		{name: "invalid returns nil", in: helpers.GetPointer[ngfAPIv1alpha1.Duration]("bogus"), expNil: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			got := convertDurationToMS(test.in)
			if test.expNil {
				g.Expect(got).To(BeNil())
				return
			}
			g.Expect(got).ToNot(BeNil())
			g.Expect(*got).To(Equal(test.expMS))
		})
	}
}

func TestConvertGraphGuardrails(t *testing.T) {
	t.Parallel()

	secretNsName := types.NamespacedName{Namespace: "ns1", Name: "token-secret"}

	validState := func() *graph.PolicyPayloadProcessorState {
		return &graph.PolicyPayloadProcessorState{
			APIURL:            "http://ext-svc.ns1.svc.cluster.local:9000",
			Timeout:           helpers.GetPointer[ngfAPIv1alpha1.Duration]("30s"),
			AuthTokenSecret:   &secretNsName,
			ResolvedAuthToken: []byte("tok"),
		}
	}

	tests := []struct {
		route      *graph.L7Route
		name       string
		expURL     string
		expNil     bool
		expFileSet bool
	}{
		{
			name:   "nil route",
			route:  nil,
			expNil: true,
		},
		{
			name:   "no effective processor",
			route:  &graph.L7Route{},
			expNil: true,
		},
		{
			name: "invalid policy",
			route: &graph.L7Route{
				EffectivePayloadProcessor: &graph.Policy{Valid: false, PayloadProcessorState: validState()},
			},
			expNil: true,
		},
		{
			name: "valid policy with token",
			route: &graph.L7Route{
				EffectivePayloadProcessor: &graph.Policy{Valid: true, PayloadProcessorState: validState()},
			},
			expURL:     "http://ext-svc.ns1.svc.cluster.local:9000",
			expFileSet: true,
		},
		{
			name: "valid policy without token",
			route: &graph.L7Route{
				EffectivePayloadProcessor: &graph.Policy{
					Valid: true,
					PayloadProcessorState: &graph.PolicyPayloadProcessorState{
						APIURL: "http://ext-svc.ns1.svc.cluster.local:9000",
					},
				},
			},
			expURL:     "http://ext-svc.ns1.svc.cluster.local:9000",
			expFileSet: false,
		},
		{
			name: "valid policy with empty APIURL",
			route: &graph.L7Route{
				EffectivePayloadProcessor: &graph.Policy{
					Valid:                 true,
					PayloadProcessorState: &graph.PolicyPayloadProcessorState{APIURL: ""},
				},
			},
			expNil: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			got := convertGraphGuardrails(test.route)
			if test.expNil {
				g.Expect(got).To(BeNil())
				return
			}

			g.Expect(got).ToNot(BeNil())
			g.Expect(got.Filter).To(Equal("on"))
			g.Expect(got.APIURL).To(Equal(test.expURL))

			if test.expFileSet {
				g.Expect(got.APITokenAuthFileID).To(Equal(GenerateGuardrailsTokenFileID("ns1", "token-secret")))
			} else {
				g.Expect(got.APITokenAuthFileID).To(BeEmpty())
			}
		})
	}
}

func TestBuildGuardrailsAuthSecrets(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	secretNsName := types.NamespacedName{Namespace: "ns1", Name: "token-secret"}

	withToken := &graph.L7Route{
		EffectivePayloadProcessor: &graph.Policy{
			Valid: true,
			PayloadProcessorState: &graph.PolicyPayloadProcessorState{
				APIURL:            "http://svc:9000",
				AuthTokenSecret:   &secretNsName,
				ResolvedAuthToken: []byte("tok"),
			},
		},
	}
	withoutToken := &graph.L7Route{
		EffectivePayloadProcessor: &graph.Policy{
			Valid:                 true,
			PayloadProcessorState: &graph.PolicyPayloadProcessorState{APIURL: "http://svc:9000"},
		},
	}
	noProcessor := &graph.L7Route{}

	routes := map[graph.RouteKey]*graph.L7Route{
		{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "r1"}}: withToken,
		{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "r2"}}: withoutToken,
		{NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "r3"}}: noProcessor,
	}

	got := buildGuardrailsAuthSecrets(&graph.Gateway{}, routes)

	id := GenerateGuardrailsTokenFileID("ns1", "token-secret")
	g.Expect(got).To(HaveLen(1))
	g.Expect(got).To(HaveKeyWithValue(id, AuthFileData("tok")))

	g.Expect(buildGuardrailsAuthSecrets(nil, routes)).To(BeEmpty())
}

func TestGuardrailsEnabled(t *testing.T) {
	t.Parallel()

	withGuardrails := []VirtualServer{
		{
			PathRules: []PathRule{
				{MatchRules: []MatchRule{{Guardrails: &GuardrailsConfig{Filter: "on"}}}},
			},
		},
	}
	withoutGuardrails := []VirtualServer{
		{PathRules: []PathRule{{MatchRules: []MatchRule{{}}}}},
	}

	g := NewWithT(t)
	g.Expect(guardrailsEnabled(withoutGuardrails)).To(BeFalse())
	g.Expect(guardrailsEnabled(withoutGuardrails, withGuardrails)).To(BeTrue())
	g.Expect(guardrailsEnabled()).To(BeFalse())
}
