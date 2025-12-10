package config

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestExecuteSplitClients(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expStrings    map[string]int
		msg           string
		notExpStrings []string
		configuration dataplane.Configuration
	}{
		{
			msg: "non-zero weights",
			configuration: dataplane.Configuration{
				BackendGroups: []dataplane.BackendGroup{
					{
						Source:  types.NamespacedName{Namespace: "test", Name: "hr"},
						RuleIdx: 0,
						Backends: []dataplane.Backend{
							{UpstreamName: "test1", Valid: true, Weight: 1},
							{UpstreamName: "test2", Valid: true, Weight: 1},
						},
					},
					{
						Source:  types.NamespacedName{Namespace: "test", Name: "no-split"},
						RuleIdx: 1,
						Backends: []dataplane.Backend{
							{UpstreamName: "no-split", Valid: true, Weight: 1},
						},
					},
					{
						Source:  types.NamespacedName{Namespace: "test", Name: "hr"},
						RuleIdx: 1,
						Backends: []dataplane.Backend{
							{UpstreamName: "test3", Valid: true, Weight: 1},
							{UpstreamName: "test4", Valid: true, Weight: 1},
						},
					},
				},
			},
			expStrings: map[string]int{
				"split_clients $request_id $group_test__hr_rule0": 1,
				"split_clients $request_id $group_test__hr_rule1": 1,
				"50.00% test1;": 1,
				"50.00% test2;": 1,
				"50.00% test3;": 1,
				"50.00% test4;": 1,
			},
			notExpStrings: []string{"no-split", "#"},
		},
		{
			msg: "zero weight",
			configuration: dataplane.Configuration{
				BackendGroups: []dataplane.BackendGroup{
					{
						Source:  types.NamespacedName{Namespace: "test", Name: "zero-percent"},
						RuleIdx: 0,
						Backends: []dataplane.Backend{
							{UpstreamName: "non-zero", Valid: true, Weight: 1},
							{UpstreamName: "zero", Valid: true, Weight: 0},
						},
					},
				},
			},
			expStrings: map[string]int{
				"split_clients $request_id $group_test__zero_percent_rule0": 1,
				"100.00% non-zero;": 1,
				"# 0.00% zero;":     1,
			},
			notExpStrings: nil,
		},
		{
			msg: "no split clients",
			configuration: dataplane.Configuration{
				BackendGroups: []dataplane.BackendGroup{
					{
						Source:  types.NamespacedName{Namespace: "test", Name: "single-backend-route"},
						RuleIdx: 0,
						Backends: []dataplane.Backend{
							{UpstreamName: "single-backend", Valid: true, Weight: 1},
						},
					},
				},
			},
			expStrings:    map[string]int{},
			notExpStrings: []string{"split_clients"},
		},
		{
			msg: "HTTPServer mirror split clients",
			configuration: dataplane.Configuration{
				HTTPServers: []dataplane.VirtualServer{
					{
						PathRules: []dataplane.PathRule{
							{
								Path: "/mirror",
								MatchRules: []dataplane.MatchRule{
									{
										Filters: dataplane.HTTPFilters{
											RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-0"),
													Percent: helpers.GetPointer(float64(25)),
												},
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-tea-backend-test/route1-0"),
													Percent: helpers.GetPointer(float64(50)),
												},
											},
										},
									},
									{
										Filters: dataplane.HTTPFilters{
											RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-1"),
													Percent: helpers.GetPointer(float64(25)),
												},
											},
										},
									},
								},
							},
							{
								Path: http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-0",
							},
							{
								Path: http.InternalMirrorRoutePathPrefix + "-my-tea-backend-test/route1-0",
							},
							{
								Path: http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-1",
							},
							{
								Path: "/mirror-edge-case-percentages",
								MatchRules: []dataplane.MatchRule{
									{
										Filters: dataplane.HTTPFilters{
											RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route2-0"),
													Percent: helpers.GetPointer(float64(0)),
												},
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-tea-backend-test/route2-0"),
													Percent: helpers.GetPointer(float64(99.999)),
												},
											},
										},
									},
									{
										Filters: dataplane.HTTPFilters{
											RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route2-1"),
													Percent: helpers.GetPointer(float64(0.001)),
												},
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-tea-backend-test/route2-1"),
													Percent: helpers.GetPointer(float64(100)),
												},
											},
										},
									},
								},
							},
							{
								Path: http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route2-0",
							},
							{
								Path: http.InternalMirrorRoutePathPrefix + "-my-tea-backend-test/route2-0",
							},
							{
								Path: http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route2-1",
							},
						},
					},
				},
			},
			expStrings: map[string]int{
				"split_clients $request_id $__ngf_internal_mirror_my_coffee_backend_test_route1_0_25_00": 1,
				"25.00% /_ngf-internal-mirror-my-coffee-backend-test/route1-0":                           1,

				"split_clients $request_id $__ngf_internal_mirror_my_tea_backend_test_route1_0_50_00": 1,
				"50.00% /_ngf-internal-mirror-my-tea-backend-test/route1-0":                           1,

				"split_clients $request_id $__ngf_internal_mirror_my_coffee_backend_test_route1_1_25_00": 1,
				"25.00% /_ngf-internal-mirror-my-coffee-backend-test/route1-1":                           1,

				"split_clients $request_id $__ngf_internal_mirror_my_coffee_backend_test_route2_0_0_00": 1,
				"0.00% /_ngf-internal-mirror-my-coffee-backend-test/route2-0":                           1,

				"split_clients $request_id $__ngf_internal_mirror_my_tea_backend_test_route2_0_100_00": 1,
				"100.00% /_ngf-internal-mirror-my-tea-backend-test/route2-0":                           1,

				"split_clients $request_id $__ngf_internal_mirror_my_coffee_backend_test_route2_1_0_00": 1,
				"0.00% /_ngf-internal-mirror-my-coffee-backend-test/route2-1":                           1,
				"* \"\"": 6,
			},
			notExpStrings: []string{
				"split_clients $request_id $__ngf_internal_mirror_my_tea_backend_test_route2_1_100_00",
			},
		},
		{
			msg: "Duplicate split clients are not created",
			configuration: dataplane.Configuration{
				HTTPServers: []dataplane.VirtualServer{
					{
						PathRules: []dataplane.PathRule{
							{
								Path: "/mirror",
								MatchRules: []dataplane.MatchRule{
									{
										Filters: dataplane.HTTPFilters{
											RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-same-backend-test/route1-0"),
													Percent: helpers.GetPointer(float64(25)),
												},
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-same-backend-test/route1-0"),
													Percent: helpers.GetPointer(float64(50)),
												},
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-same-backend-test/route1-0"),
													Percent: helpers.GetPointer(float64(50)),
												},
											},
										},
									},
								},
							},
							{
								Path: http.InternalMirrorRoutePathPrefix + "-my-same-backend-test/route1-0",
							},
						},
					},
				},
			},
			expStrings: map[string]int{
				"split_clients $request_id $__ngf_internal_mirror_my_same_backend_test_route1_0_50_00": 1,
				"50.00% /_ngf-internal-mirror-my-same-backend-test/route1-0":                           1,
				"* \"\"": 1,
			},
			notExpStrings: []string{
				"split_clients $request_id $__ngf_internal_mirror_my_coffee_backend_test_route1_0_25_00",
				"25.00% /_ngf-internal-mirror-my-same-backend-test/route1-0",
			},
		},
		{
			msg: "BackendGroup and Server split clients",
			configuration: dataplane.Configuration{
				BackendGroups: []dataplane.BackendGroup{
					{
						Source:  types.NamespacedName{Namespace: "test", Name: "hr"},
						RuleIdx: 0,
						Backends: []dataplane.Backend{
							{UpstreamName: "test1", Valid: true, Weight: 1},
							{UpstreamName: "test2", Valid: true, Weight: 1},
						},
					},
					{
						Source:  types.NamespacedName{Namespace: "test", Name: "hr"},
						RuleIdx: 1,
						Backends: []dataplane.Backend{
							{UpstreamName: "test3", Valid: true, Weight: 1},
							{UpstreamName: "test4", Valid: true, Weight: 1},
						},
					},
				},
				HTTPServers: []dataplane.VirtualServer{
					{
						PathRules: []dataplane.PathRule{
							{
								Path: "/mirror",
								MatchRules: []dataplane.MatchRule{
									{
										Filters: dataplane.HTTPFilters{
											RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-backend-test/route1-0"),
													Percent: helpers.GetPointer(float64(25)),
												},
											},
										},
									},
								},
							},
							{
								Path: http.InternalMirrorRoutePathPrefix + "-my-backend-test/route1-0",
							},
						},
					},
				},
				SSLServers: []dataplane.VirtualServer{
					{
						PathRules: []dataplane.PathRule{
							{
								Path: "/mirror-ssl",
								MatchRules: []dataplane.MatchRule{
									{
										Filters: dataplane.HTTPFilters{
											RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
												{
													Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-ssl-backend-test/route1-0"),
													Percent: helpers.GetPointer(float64(50)),
												},
											},
										},
									},
								},
							},
							{
								Path: http.InternalMirrorRoutePathPrefix + "-my-ssl-backend-test/route1-0",
							},
						},
					},
				},
			},
			expStrings: map[string]int{
				"split_clients $request_id $group_test__hr_rule0": 1,
				"split_clients $request_id $group_test__hr_rule1": 1,
				"50.00% test1;": 1,
				"50.00% test2;": 1,
				"50.00% test3;": 1,
				"50.00% test4;": 1,
				"split_clients $request_id $__ngf_internal_mirror_my_backend_test_route1_0_25_00":     1,
				"25.00% /_ngf-internal-mirror-my-backend-test/route1-0":                               1,
				"split_clients $request_id $__ngf_internal_mirror_my_ssl_backend_test_route1_0_50_00": 1,
				"50.00% /_ngf-internal-mirror-my-ssl-backend-test/route1-0":                           1,
				"* \"\"": 2,
			},
			notExpStrings: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			splitResults := executeSplitClients(test.configuration)

			g.Expect(splitResults).To(HaveLen(1))
			g.Expect(splitResults[0].dest).To(Equal(httpConfigFile))

			sc := string(splitResults[0].data)

			for expSubStr, expCount := range test.expStrings {
				g.Expect(strings.Count(sc, expSubStr)).To(Equal(expCount))
			}

			for _, notExpString := range test.notExpStrings {
				g.Expect(sc).ToNot(ContainSubstring(notExpString))
			}
		})
	}
}

func TestCreateRequestMirrorSplitClients(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg             string
		servers         []dataplane.VirtualServer
		expSplitClients []http.SplitClient
	}{
		{
			msg: "normal case",
			servers: []dataplane.VirtualServer{
				{
					PathRules: []dataplane.PathRule{
						{
							Path: "/mirror",
							MatchRules: []dataplane.MatchRule{
								{
									Filters: dataplane.HTTPFilters{
										RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
											{
												Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-0"),
												Percent: helpers.GetPointer(float64(25)),
											},
											{
												Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-tea-backend-test/route1-0"),
												Percent: helpers.GetPointer(float64(50)),
											},
										},
									},
								},
								{
									Filters: dataplane.HTTPFilters{
										RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
											{
												Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-1"),
												Percent: helpers.GetPointer(float64(25)),
											},
										},
									},
								},
							},
						},
						{
							Path: http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-0",
						},
						{
							Path: http.InternalMirrorRoutePathPrefix + "-my-tea-backend-test/route1-0",
						},
						{
							Path: http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-1",
						},
					},
				},
				{
					PathRules: []dataplane.PathRule{
						{
							Path: "/mirror-different-server",
							MatchRules: []dataplane.MatchRule{
								{
									Filters: dataplane.HTTPFilters{
										RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
											{
												Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-0"),
												Percent: helpers.GetPointer(float64(30)),
											},
										},
									},
								},
							},
						},
						{
							Path: http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-0",
						},
					},
				},
			},
			expSplitClients: []http.SplitClient{
				{
					VariableName: "__ngf_internal_mirror_my_coffee_backend_test_route1_0_25_00",
					Distributions: []http.SplitClientDistribution{
						{
							Percent: "25.00",
							Value:   "/_ngf-internal-mirror-my-coffee-backend-test/route1-0",
						},
						{
							Percent: "*",
							Value:   "\"\"",
						},
					},
				},
				{
					VariableName: "__ngf_internal_mirror_my_tea_backend_test_route1_0_50_00",
					Distributions: []http.SplitClientDistribution{
						{
							Percent: "50.00",
							Value:   "/_ngf-internal-mirror-my-tea-backend-test/route1-0",
						},
						{
							Percent: "*",
							Value:   "\"\"",
						},
					},
				},
				{
					VariableName: "__ngf_internal_mirror_my_coffee_backend_test_route1_1_25_00",
					Distributions: []http.SplitClientDistribution{
						{
							Percent: "25.00",
							Value:   "/_ngf-internal-mirror-my-coffee-backend-test/route1-1",
						},
						{
							Percent: "*",
							Value:   "\"\"",
						},
					},
				},
				{
					VariableName: "__ngf_internal_mirror_my_coffee_backend_test_route1_0_30_00",
					Distributions: []http.SplitClientDistribution{
						{
							Percent: "30.00",
							Value:   "/_ngf-internal-mirror-my-coffee-backend-test/route1-0",
						},
						{
							Percent: "*",
							Value:   "\"\"",
						},
					},
				},
			},
		},
		{
			msg: "no split clients are needed",
			servers: []dataplane.VirtualServer{
				{
					PathRules: []dataplane.PathRule{
						{
							Path: "/mirror",
							MatchRules: []dataplane.MatchRule{
								{
									Filters: dataplane.HTTPFilters{
										RequestMirrors: []*dataplane.HTTPRequestMirrorFilter{
											{
												Target:  helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-0"),
												Percent: helpers.GetPointer(float64(100)),
											},
											{
												Target: helpers.GetPointer(http.InternalMirrorRoutePathPrefix + "-my-tea-backend-test/route1-0"),
											},
											{
												Target: helpers.GetPointer("path-does-not-exist"),
											},
										},
									},
								},
							},
						},
						{
							Path: http.InternalMirrorRoutePathPrefix + "-my-coffee-backend-test/route1-0",
						},
						{
							Path: http.InternalMirrorRoutePathPrefix + "-my-tea-backend-test/route1-0",
						},
					},
				},
			},
			expSplitClients: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := createRequestMirrorSplitClients(test.servers)
			g.Expect(result).To(ContainElements(test.expSplitClients))
		})
	}
}

func TestBackendGroupCreateSplitClients(t *testing.T) {
	t.Parallel()
	hrNoSplit := types.NamespacedName{Namespace: "test", Name: "hr-no-split"}
	hrOneSplit := types.NamespacedName{Namespace: "test", Name: "hr-one-split"}
	hrTwoSplits := types.NamespacedName{Namespace: "test", Name: "hr-two-splits"}

	createBackendGroup := func(
		sourceNsName types.NamespacedName,
		ruleIdx int,
		backends ...dataplane.Backend,
	) dataplane.BackendGroup {
		return dataplane.BackendGroup{
			Source:   sourceNsName,
			RuleIdx:  ruleIdx,
			Backends: backends,
		}
	}
	// the following backends do not need splits
	noBackends := createBackendGroup(hrNoSplit, 0)

	oneBackend := createBackendGroup(
		hrNoSplit,
		0,
		dataplane.Backend{UpstreamName: "one-backend", Valid: true, Weight: 1},
	)

	invalidBackend := createBackendGroup(
		hrNoSplit,
		0,
		dataplane.Backend{UpstreamName: "invalid-backend", Valid: false, Weight: 1},
	)

	// the following backends need splits
	oneSplit := createBackendGroup(
		hrOneSplit,
		0,
		dataplane.Backend{UpstreamName: "one-split-1", Valid: true, Weight: 50},
		dataplane.Backend{UpstreamName: "one-split-2", Valid: true, Weight: 50},
	)

	twoSplitGroup0 := createBackendGroup(
		hrTwoSplits,
		0,
		dataplane.Backend{UpstreamName: "two-split-1", Valid: true, Weight: 50},
		dataplane.Backend{UpstreamName: "two-split-2", Valid: true, Weight: 50},
	)

	twoSplitGroup1 := createBackendGroup(
		hrTwoSplits,
		1,
		dataplane.Backend{UpstreamName: "two-split-3", Valid: true, Weight: 50},
		dataplane.Backend{UpstreamName: "two-split-4", Valid: true, Weight: 50},
		dataplane.Backend{UpstreamName: "two-split-5", Valid: true, Weight: 50},
	)

	tests := []struct {
		msg             string
		backendGroups   []dataplane.BackendGroup
		expSplitClients []http.SplitClient
	}{
		{
			msg: "normal case",
			backendGroups: []dataplane.BackendGroup{
				noBackends,
				oneBackend,
				invalidBackend,
				oneSplit,
				twoSplitGroup0,
				twoSplitGroup1,
			},
			expSplitClients: []http.SplitClient{
				{
					VariableName: "group_test__hr_one_split_rule0",
					Distributions: []http.SplitClientDistribution{
						{
							Percent: "50.00",
							Value:   "one-split-1",
						},
						{
							Percent: "50.00",
							Value:   "one-split-2",
						},
					},
				},
				{
					VariableName: "group_test__hr_two_splits_rule0",
					Distributions: []http.SplitClientDistribution{
						{
							Percent: "50.00",
							Value:   "two-split-1",
						},
						{
							Percent: "50.00",
							Value:   "two-split-2",
						},
					},
				},
				{
					VariableName: "group_test__hr_two_splits_rule1",
					Distributions: []http.SplitClientDistribution{
						{
							Percent: "33.33",
							Value:   "two-split-3",
						},
						{
							Percent: "33.33",
							Value:   "two-split-4",
						},
						{
							Percent: "33.34",
							Value:   "two-split-5",
						},
					},
				},
			},
		},
		{
			msg: "no split clients are needed",
			backendGroups: []dataplane.BackendGroup{
				noBackends,
				oneBackend,
			},
			expSplitClients: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := createBackendGroupSplitClients(test.backendGroups)
			g.Expect(result).To(Equal(test.expSplitClients))
		})
	}
}

func TestCreateBackendGroupSplitClientDistributions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg              string
		backends         []dataplane.Backend
		expDistributions []http.SplitClientDistribution
	}{
		{
			msg:              "no backends",
			backends:         nil,
			expDistributions: nil,
		},
		{
			msg: "one backend",
			backends: []dataplane.Backend{
				{
					UpstreamName: "one",
					Valid:        true,
					Weight:       1,
				},
			},
			expDistributions: nil,
		},
		{
			msg: "total weight 0",
			backends: []dataplane.Backend{
				{
					UpstreamName: "one",
					Valid:        true,
					Weight:       0,
				},
				{
					UpstreamName: "two",
					Valid:        true,
					Weight:       0,
				},
			},
			expDistributions: []http.SplitClientDistribution{
				{
					Percent: "100",
					Value:   invalidBackendRef,
				},
			},
		},
		{
			msg: "two backends; equal weights that sum to 100",
			backends: []dataplane.Backend{
				{
					UpstreamName: "one",
					Valid:        true,
					Weight:       1,
				},
				{
					UpstreamName: "two",
					Valid:        true,
					Weight:       1,
				},
			},
			expDistributions: []http.SplitClientDistribution{
				{
					Percent: "50.00",
					Value:   "one",
				},
				{
					Percent: "50.00",
					Value:   "two",
				},
			},
		},
		{
			msg: "three backends; whole percentages that sum to 100",
			backends: []dataplane.Backend{
				{
					UpstreamName: "one",
					Valid:        true,
					Weight:       20,
				},
				{
					UpstreamName: "two",
					Valid:        true,
					Weight:       30,
				},
				{
					UpstreamName: "three",
					Valid:        true,
					Weight:       50,
				},
			},
			expDistributions: []http.SplitClientDistribution{
				{
					Percent: "20.00",
					Value:   "one",
				},
				{
					Percent: "30.00",
					Value:   "two",
				},
				{
					Percent: "50.00",
					Value:   "three",
				},
			},
		},
		{
			msg: "three backends; whole percentages that sum to less than 100",
			backends: []dataplane.Backend{
				{
					UpstreamName: "one",
					Valid:        true,
					Weight:       3,
				},
				{
					UpstreamName: "two",
					Valid:        true,
					Weight:       3,
				},
				{
					UpstreamName: "three",
					Valid:        true,
					Weight:       3,
				},
			},
			expDistributions: []http.SplitClientDistribution{
				{
					Percent: "33.33",
					Value:   "one",
				},
				{
					Percent: "33.33",
					Value:   "two",
				},
				{
					Percent: "33.34", // the last backend gets the remainder.
					Value:   "three",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := createBackendGroupSplitClientDistributions(dataplane.BackendGroup{Backends: test.backends})
			g.Expect(result).To(Equal(test.expDistributions))
		})
	}
}

func TestGetSplitClientValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg      string
		expValue string
		backend  dataplane.Backend
	}{
		{
			msg: "valid backend",
			backend: dataplane.Backend{
				UpstreamName: "valid",
				Valid:        true,
			},
			expValue: "valid",
		},
		{
			msg: "invalid backend",
			backend: dataplane.Backend{
				UpstreamName: "invalid",
				Valid:        false,
			},
			expValue: invalidBackendRef,
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := getSplitClientValue(test.backend, types.NamespacedName{Namespace: "test", Name: "hr"}, 0)
			g.Expect(result).To(Equal(test.expValue))
		})
	}
}

func TestPercentOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg         string
		weight      int32
		totalWeight int32
		expPercent  float64
	}{
		{
			msg:         "50/100",
			weight:      50,
			totalWeight: 100,
			expPercent:  50,
		},
		{
			msg:         "2000/4000",
			weight:      2000,
			totalWeight: 4000,
			expPercent:  50,
		},
		{
			msg:         "100/100",
			weight:      100,
			totalWeight: 100,
			expPercent:  100,
		},
		{
			msg:         "5/5",
			weight:      5,
			totalWeight: 5,
			expPercent:  100,
		},
		{
			msg:         "0/8000",
			weight:      0,
			totalWeight: 8000,
			expPercent:  0,
		},
		{
			msg:         "2/3",
			weight:      2,
			totalWeight: 3,
			expPercent:  66.66,
		},
		{
			msg:         "4/15",
			weight:      4,
			totalWeight: 15,
			expPercent:  26.66,
		},
		{
			msg:         "800/2000",
			weight:      800,
			totalWeight: 2000,
			expPercent:  40,
		},
		{
			msg:         "300/2400",
			weight:      300,
			totalWeight: 2400,
			expPercent:  12.5,
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			percent := percentOf(test.weight, test.totalWeight)
			g.Expect(percent).To(Equal(test.expPercent))
		})
	}
}

func TestBackendGroupNeedsSplit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg      string
		backends []dataplane.Backend
		expSplit bool
	}{
		{
			msg:      "empty backends",
			backends: []dataplane.Backend{},
			expSplit: false,
		},
		{
			msg:      "nil backends",
			backends: nil,
			expSplit: false,
		},
		{
			msg: "one valid backend",
			backends: []dataplane.Backend{
				{
					UpstreamName: "backend1",
					Valid:        true,
					Weight:       1,
				},
			},
			expSplit: false,
		},
		{
			msg: "one invalid backend",
			backends: []dataplane.Backend{
				{
					UpstreamName: "backend1",
					Valid:        false,
					Weight:       1,
				},
			},
			expSplit: false,
		},
		{
			msg: "multiple valid backends",
			backends: []dataplane.Backend{
				{
					UpstreamName: "backend1",
					Valid:        true,
					Weight:       1,
				},
				{
					UpstreamName: "backend2",
					Valid:        true,
					Weight:       1,
				},
			},
			expSplit: true,
		},
		{
			msg: "multiple backends - one invalid",
			backends: []dataplane.Backend{
				{
					UpstreamName: "backend1",
					Valid:        true,
					Weight:       1,
				},
				{
					UpstreamName: "backend2",
					Valid:        false,
					Weight:       1,
				},
			},
			expSplit: true,
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			bg := dataplane.BackendGroup{
				Source:   types.NamespacedName{Namespace: "test", Name: "hr"},
				Backends: test.backends,
			}
			result := backendGroupNeedsSplit(bg)
			g.Expect(result).To(Equal(test.expSplit))
		})
	}
}

func TestBackendGroupName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg      string
		expName  string
		backends []dataplane.Backend
	}{
		{
			msg:      "empty backends",
			backends: []dataplane.Backend{},
			expName:  invalidBackendRef,
		},
		{
			msg:      "nil backends",
			backends: nil,
			expName:  invalidBackendRef,
		},
		{
			msg: "one valid backend with non-zero weight",
			backends: []dataplane.Backend{
				{
					UpstreamName: "backend1",
					Valid:        true,
					Weight:       1,
				},
			},
			expName: "backend1",
		},
		{
			msg: "one valid backend with zero weight",
			backends: []dataplane.Backend{
				{
					UpstreamName: "backend1",
					Valid:        true,
					Weight:       0,
				},
			},
			expName: invalidBackendRef,
		},
		{
			msg: "one invalid backend",
			backends: []dataplane.Backend{
				{
					UpstreamName: "backend1",
					Valid:        false,
					Weight:       1,
				},
			},
			expName: invalidBackendRef,
		},
		{
			msg: "multiple valid backends",
			backends: []dataplane.Backend{
				{
					UpstreamName: "backend1",
					Valid:        true,
					Weight:       1,
				},
				{
					UpstreamName: "backend2",
					Valid:        true,
					Weight:       1,
				},
			},
			expName: "group_test__hr_rule0",
		},
		{
			msg: "multiple invalid backends",
			backends: []dataplane.Backend{
				{
					UpstreamName: "backend1",
					Valid:        false,
					Weight:       1,
				},
				{
					UpstreamName: "backend2",
					Valid:        false,
					Weight:       1,
				},
			},
			expName: "group_test__hr_rule0",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			bg := dataplane.BackendGroup{
				Source:   types.NamespacedName{Namespace: "test", Name: "hr"},
				RuleIdx:  0,
				Backends: test.backends,
			}
			result := backendGroupName(bg)
			g.Expect(result).To(Equal(test.expName))
		})
	}
}
