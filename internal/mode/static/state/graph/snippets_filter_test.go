package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

func TestProcessSnippetsFilters(t *testing.T) {
	t.Parallel()

	filter1NsName := types.NamespacedName{Namespace: "test", Name: "filter-1"}
	filter2NsName := types.NamespacedName{Namespace: "other", Name: "filter-2"}
	invalidFilterNsName := types.NamespacedName{Namespace: "default", Name: "invalid"}

	filter1 := &ngfAPI.SnippetsFilter{
		Spec: ngfAPI.SnippetsFilterSpec{
			Snippets: []ngfAPI.Snippet{
				{
					Context: ngfAPI.NginxContextMain,
					Value:   "main snippet",
				},
				{
					Context: ngfAPI.NginxContextHTTP,
					Value:   "http snippet",
				},
			},
		},
	}

	invalidFilter := &ngfAPI.SnippetsFilter{
		Spec: ngfAPI.SnippetsFilterSpec{
			Snippets: []ngfAPI.Snippet{
				{
					Context: ngfAPI.NginxContextMain,
					Value:   "main snippet",
				},
				{
					Context: "invalid context",
					Value:   "invalid snippet",
				},
			},
		},
	}

	filter2 := &ngfAPI.SnippetsFilter{
		Spec: ngfAPI.SnippetsFilterSpec{
			Snippets: []ngfAPI.Snippet{
				{
					Context: ngfAPI.NginxContextHTTPServerLocation,
					Value:   "location snippet",
				},
			},
		},
	}

	tests := []struct {
		snippetsFilters      map[types.NamespacedName]*ngfAPI.SnippetsFilter
		expProcessedSnippets map[types.NamespacedName]*SnippetsFilter
		msg                  string
	}{
		{
			msg:                  "no snippets filters",
			snippetsFilters:      nil,
			expProcessedSnippets: nil,
		},
		{
			msg: "mix valid and invalid snippets filters",
			snippetsFilters: map[types.NamespacedName]*ngfAPI.SnippetsFilter{
				filter1NsName:       filter1,
				invalidFilterNsName: invalidFilter,
				filter2NsName:       filter2,
			},
			expProcessedSnippets: map[types.NamespacedName]*SnippetsFilter{
				filter1NsName: {
					Source:     filter1,
					Conditions: nil,
					Valid:      true,
					Referenced: false,
					Snippets: map[ngfAPI.NginxContext]string{
						ngfAPI.NginxContextMain: "main snippet",
						ngfAPI.NginxContextHTTP: "http snippet",
					},
				},
				filter2NsName: {
					Source:     filter2,
					Conditions: nil,
					Valid:      true,
					Referenced: false,
					Snippets: map[ngfAPI.NginxContext]string{
						ngfAPI.NginxContextHTTPServerLocation: "location snippet",
					},
				},
				invalidFilterNsName: {
					Source: invalidFilter,
					Conditions: []conditions.Condition{
						staticConds.NewSnippetsFilterInvalid(
							"spec.snippets[1].context: Unsupported value: \"invalid context\": " +
								"supported values: \"main\", \"http\", \"http.server\", \"http.server.location\"",
						),
					},
					Valid: false,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			processedSnippetsFilters := processSnippetsFilters(test.snippetsFilters)
			g.Expect(processedSnippetsFilters).To(BeEquivalentTo(test.expProcessedSnippets))
		})
	}
}

func TestValidateSnippetsFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg     string
		filter  *ngfAPI.SnippetsFilter
		expCond conditions.Condition
	}{
		{
			msg: "valid filter",
			filter: &ngfAPI.SnippetsFilter{
				Spec: ngfAPI.SnippetsFilterSpec{
					Snippets: []ngfAPI.Snippet{
						{
							Context: ngfAPI.NginxContextMain,
							Value:   "main snippet",
						},
						{
							Context: ngfAPI.NginxContextHTTP,
							Value:   "http snippet",
						},
					},
				},
			},
			expCond: conditions.Condition{},
		},
		{
			msg:    "empty filter",
			filter: &ngfAPI.SnippetsFilter{},
			expCond: staticConds.NewSnippetsFilterInvalid(
				"spec.snippets: Required value: at least one snippet must be provided",
			),
		},
		{
			msg: "invalid filter; invalid snippet context",
			filter: &ngfAPI.SnippetsFilter{
				Spec: ngfAPI.SnippetsFilterSpec{
					Snippets: []ngfAPI.Snippet{
						{
							Context: ngfAPI.NginxContextMain,
							Value:   "main snippet",
						},
						{
							Context: ngfAPI.NginxContextHTTP,
							Value:   "http snippet",
						},
						{
							Context: "invalid context",
							Value:   "invalid",
						},
					},
				},
			},
			expCond: staticConds.NewSnippetsFilterInvalid(
				"spec.snippets[2].context: Unsupported value: \"invalid context\": " +
					"supported values: \"main\", \"http\", \"http.server\", \"http.server.location\"",
			),
		},
		{
			msg: "invalid filter; multiple invalid snippet contexts",
			filter: &ngfAPI.SnippetsFilter{
				Spec: ngfAPI.SnippetsFilterSpec{
					Snippets: []ngfAPI.Snippet{
						{
							Context: ngfAPI.NginxContextMain,
							Value:   "main snippet",
						},
						{
							Context: "invalid context",
							Value:   "invalid",
						},
						{
							Context: "", // empty context
							Value:   "invalid too",
						},
					},
				},
			},
			expCond: staticConds.NewSnippetsFilterInvalid(
				"[spec.snippets[1].context: Unsupported value: \"invalid context\": supported values: " +
					"\"main\", \"http\", \"http.server\", \"http.server.location\", spec.snippets[2].context: " +
					"Unsupported value: \"\": supported values: \"main\", \"http\", " +
					"\"http.server\", \"http.server.location\"]",
			),
		},
		{
			msg: "invalid filter; duplicate contexts",
			filter: &ngfAPI.SnippetsFilter{
				Spec: ngfAPI.SnippetsFilterSpec{
					Snippets: []ngfAPI.Snippet{
						{
							Context: ngfAPI.NginxContextMain,
							Value:   "main snippet",
						},
						{
							Context: ngfAPI.NginxContextHTTP,
							Value:   "http snippet",
						},
						{
							Context: ngfAPI.NginxContextMain,
							Value:   "main again",
						},
					},
				},
			},
			expCond: staticConds.NewSnippetsFilterInvalid(
				"spec.snippets[2].context: Invalid value: \"main\": only one snippet is allowed per context",
			),
		},
		{
			msg: "invalid filter; duplicate contexts and invalid context",
			filter: &ngfAPI.SnippetsFilter{
				Spec: ngfAPI.SnippetsFilterSpec{
					Snippets: []ngfAPI.Snippet{
						{
							Context: ngfAPI.NginxContextMain,
							Value:   "main snippet",
						},
						{
							Context: ngfAPI.NginxContextHTTP,
							Value:   "http snippet",
						},
						{
							Context: ngfAPI.NginxContextMain,
							Value:   "main again",
						},
						{
							Context: "invalid context",
							Value:   "invalid",
						},
					},
				},
			},
			expCond: staticConds.NewSnippetsFilterInvalid(
				"[spec.snippets[2].context: Invalid value: \"main\": only one snippet is allowed per context, " +
					"spec.snippets[3].context: Unsupported value: \"invalid context\": supported values: \"main\", " +
					"\"http\", \"http.server\", \"http.server.location\"]",
			),
		},
		{
			msg: "invalid filter; empty value",
			filter: &ngfAPI.SnippetsFilter{
				Spec: ngfAPI.SnippetsFilterSpec{
					Snippets: []ngfAPI.Snippet{
						{
							Context: ngfAPI.NginxContextMain,
							Value:   "main snippet",
						},
						{
							Context: ngfAPI.NginxContextMain,
							Value:   "", // empty value
						},
					},
				},
			},
			expCond: staticConds.NewSnippetsFilterInvalid(
				"spec.snippets[1].value: Required value: value cannot be empty",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			cond := validateSnippetsFilter(test.filter)
			if test.expCond != (conditions.Condition{}) {
				g.Expect(cond).ToNot(BeNil())
				g.Expect(*cond).To(Equal(test.expCond))
			} else {
				g.Expect(cond).To(BeNil())
			}
		})
	}
}

func TestGetSnippetsFilterResolverForNamespace(t *testing.T) {
	t.Parallel()

	defaultSf1NsName := types.NamespacedName{Name: "sf1", Namespace: "default"}
	fooSf1NsName := types.NamespacedName{Name: "sf1", Namespace: "foo"}
	fooSf2InvalidNsName := types.NamespacedName{Name: "sf2-invalid", Namespace: "foo"}

	createSnippetsFilter := func(nsname types.NamespacedName, valid bool) *SnippetsFilter {
		return &SnippetsFilter{
			Source: &ngfAPI.SnippetsFilter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nsname.Name,
					Namespace: nsname.Namespace,
				},
			},
			Valid: valid,
		}
	}

	createSnippetsFilterMap := func() map[types.NamespacedName]*SnippetsFilter {
		return map[types.NamespacedName]*SnippetsFilter{
			defaultSf1NsName:    createSnippetsFilter(defaultSf1NsName, true),
			fooSf1NsName:        createSnippetsFilter(fooSf1NsName, true),
			fooSf2InvalidNsName: createSnippetsFilter(fooSf2InvalidNsName, false),
		}
	}

	tests := []struct {
		name               string
		extRef             v1.LocalObjectReference
		snippetsFilterMap  map[types.NamespacedName]*SnippetsFilter
		resolveInNamespace string
		expResolve         bool
		expValid           bool
	}{
		{
			name:               "empty ref",
			extRef:             v1.LocalObjectReference{},
			snippetsFilterMap:  createSnippetsFilterMap(),
			resolveInNamespace: "default",
			expResolve:         false,
		},
		{
			name: "no snippets filters",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.SnippetsFilter,
				Name:  v1.ObjectName(fooSf1NsName.Name),
			},
			snippetsFilterMap:  nil,
			resolveInNamespace: "default",
			expResolve:         false,
		},
		{
			name: "invalid group",
			extRef: v1.LocalObjectReference{
				Group: "invalid",
				Kind:  kinds.SnippetsFilter,
				Name:  v1.ObjectName(defaultSf1NsName.Name),
			},
			snippetsFilterMap:  createSnippetsFilterMap(),
			resolveInNamespace: "default",
			expResolve:         false,
		},
		{
			name: "invalid kind",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.Gateway,
				Name:  v1.ObjectName(defaultSf1NsName.Name),
			},
			snippetsFilterMap:  createSnippetsFilterMap(),
			resolveInNamespace: "default",
			expResolve:         false,
		},
		{
			name: "snippets filter does not exist",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.SnippetsFilter,
				Name:  v1.ObjectName("dne"),
			},
			snippetsFilterMap:  createSnippetsFilterMap(),
			resolveInNamespace: "default",
			expResolve:         false,
		},
		{
			name: "valid snippets filter exists - namespace default",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.SnippetsFilter,
				Name:  v1.ObjectName(defaultSf1NsName.Name),
			},
			snippetsFilterMap:  createSnippetsFilterMap(),
			resolveInNamespace: "default",
			expResolve:         true,
			expValid:           true,
		},
		{
			name: "valid snippets filter exists - namespace foo",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.SnippetsFilter,
				Name:  v1.ObjectName(fooSf1NsName.Name),
			},
			snippetsFilterMap:  createSnippetsFilterMap(),
			resolveInNamespace: "foo",
			expResolve:         true,
			expValid:           true,
		},
		{
			name: "invalid snippets filter exists - namespace foo",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.SnippetsFilter,
				Name:  v1.ObjectName(fooSf2InvalidNsName.Name),
			},
			snippetsFilterMap:  createSnippetsFilterMap(),
			resolveInNamespace: "foo",
			expResolve:         true,
			expValid:           false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			resolve := getSnippetsFilterResolverForNamespace(test.snippetsFilterMap, test.resolveInNamespace)
			resolvedSf := resolve(test.extRef)
			if test.expResolve {
				g.Expect(resolvedSf).ToNot(BeNil())
				g.Expect(resolvedSf.SnippetsFilter).ToNot(BeNil())
				g.Expect(resolvedSf.SnippetsFilter.Referenced).To(BeTrue())
				g.Expect(resolvedSf.SnippetsFilter.Source.Name).To(BeEquivalentTo(test.extRef.Name))
				g.Expect(resolvedSf.SnippetsFilter.Source.Namespace).To(Equal(test.resolveInNamespace))
				g.Expect(resolvedSf.Valid).To(BeEquivalentTo(test.expValid))
			} else {
				g.Expect(resolvedSf).To(BeNil())
			}
		})
	}
}
