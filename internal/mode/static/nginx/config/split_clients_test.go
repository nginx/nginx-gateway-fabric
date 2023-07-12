package config

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/nginx/config/http"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/state/dataplane"
)

func TestExecuteSplitClients(t *testing.T) {
	bg1 := dataplane.BackendGroup{
		Source:  types.NamespacedName{Namespace: "test", Name: "hr"},
		RuleIdx: 0,
		Backends: []dataplane.Backend{
			{UpstreamName: "test1", Valid: true, Weight: 1},
			{UpstreamName: "test2", Valid: true, Weight: 1},
		},
	}

	bg2 := dataplane.BackendGroup{
		Source:  types.NamespacedName{Namespace: "test", Name: "no-split"},
		RuleIdx: 1,
		Backends: []dataplane.Backend{
			{UpstreamName: "no-split", Valid: true, Weight: 1},
		},
	}

	bg3 := dataplane.BackendGroup{
		Source:  types.NamespacedName{Namespace: "test", Name: "hr"},
		RuleIdx: 1,
		Backends: []dataplane.Backend{
			{UpstreamName: "test3", Valid: true, Weight: 1},
			{UpstreamName: "test4", Valid: true, Weight: 1},
		},
	}

	tests := []struct {
		msg           string
		backendGroups []dataplane.BackendGroup
		expStrings    []string
		notExpStrings []string
	}{
		{
			msg: "non-zero weights",
			backendGroups: []dataplane.BackendGroup{
				bg1,
				bg2,
				bg3,
			},
			expStrings: []string{
				"split_clients $request_id $test__hr_rule0",
				"split_clients $request_id $test__hr_rule1",
				"50.00% test1;",
				"50.00% test2;",
				"50.00% test3;",
				"50.00% test4;",
			},
			notExpStrings: []string{"no-split", "#"},
		},
		{
			msg: "zero weight",
			backendGroups: []dataplane.BackendGroup{
				{
					Source:  types.NamespacedName{Namespace: "test", Name: "zero-percent"},
					RuleIdx: 0,
					Backends: []dataplane.Backend{
						{UpstreamName: "non-zero", Valid: true, Weight: 1},
						{UpstreamName: "zero", Valid: true, Weight: 0},
					},
				},
			},
			expStrings: []string{
				"split_clients $request_id $test__zero_percent_rule0",
				"100.00% non-zero;",
				"# 0.00% zero;",
			},
			notExpStrings: nil,
		},
		{
			msg: "no split clients",
			backendGroups: []dataplane.BackendGroup{
				{
					Source:  types.NamespacedName{Namespace: "test", Name: "single-backend-route"},
					RuleIdx: 0,
					Backends: []dataplane.Backend{
						{UpstreamName: "single-backend", Valid: true, Weight: 1},
					},
				},
			},
			expStrings:    []string{},
			notExpStrings: []string{"split_clients"},
		},
	}

	for _, test := range tests {
		sc := string(executeSplitClients(dataplane.Configuration{BackendGroups: test.backendGroups}))

		for _, expSubString := range test.expStrings {
			if !strings.Contains(sc, expSubString) {
				t.Errorf(
					"executeSplitClients() did not generate split clients with substring %q for test %q. Got: %v",
					expSubString,
					test.msg,
					sc,
				)
			}
		}

		for _, notExpString := range test.notExpStrings {
			if strings.Contains(sc, notExpString) {
				t.Errorf(
					"executeSplitClients() generated split clients with unexpected substring %q for test %q. Got: %v",
					notExpString,
					test.msg,
					sc,
				)
			}
		}
	}
}

func TestCreateSplitClients(t *testing.T) {
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
					VariableName: "test__hr_one_split_rule0",
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
					VariableName: "test__hr_two_splits_rule0",
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
					VariableName: "test__hr_two_splits_rule1",
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
		result := createSplitClients(test.backendGroups)
		if diff := cmp.Diff(test.expSplitClients, result); diff != "" {
			t.Errorf("createSplitClients() mismatch for %q (-want +got):\n%s", test.msg, diff)
		}
	}
}

func TestCreateSplitClientDistributions(t *testing.T) {
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
		result := createSplitClientDistributions(dataplane.BackendGroup{Backends: test.backends})
		if diff := cmp.Diff(test.expDistributions, result); diff != "" {
			t.Errorf("createSplitClientDistributions() mismatch for %q (-want +got):\n%s", test.msg, diff)
		}
	}
}

func TestGetSplitClientValue(t *testing.T) {
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
		result := getSplitClientValue(test.backend)
		if result != test.expValue {
			t.Errorf(
				"getSplitClientValue() mismatch for %q; expected %s, got %s",
				test.msg, test.expValue, result,
			)
		}
	}
}

func TestPercentOf(t *testing.T) {
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
		percent := percentOf(test.weight, test.totalWeight)
		if percent != test.expPercent {
			t.Errorf(
				"percentOf() mismatch for test %q; expected %f, got %f",
				test.msg, test.expPercent, percent,
			)
		}
	}
}

func TestBackendGroupNeedsSplit(t *testing.T) {
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
		bg := dataplane.BackendGroup{
			Source:   types.NamespacedName{Namespace: "test", Name: "hr"},
			Backends: test.backends,
		}
		result := backendGroupNeedsSplit(bg)
		if result != test.expSplit {
			t.Errorf("backendGroupNeedsSplit() mismatch for %q; expected %t", test.msg, result)
		}
	}
}

func TestBackendGroupName(t *testing.T) {
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
			expName: "test__hr_rule0",
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
			expName: "test__hr_rule0",
		},
	}

	for _, test := range tests {
		bg := dataplane.BackendGroup{
			Source:   types.NamespacedName{Namespace: "test", Name: "hr"},
			RuleIdx:  0,
			Backends: test.backends,
		}
		result := backendGroupName(bg)
		if result != test.expName {
			t.Errorf("backendGroupName() mismatch for %q; expected %s, got %s", test.msg, test.expName, result)
		}
	}
}
