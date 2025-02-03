package config

import (
	"fmt"
	"math"
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

var splitClientsTemplate = gotemplate.Must(gotemplate.New("split_clients").Parse(splitClientsTemplateText))

func executeSplitClients(conf dataplane.Configuration) []executeResult {
	splitClients := createSplitClients(conf.BackendGroups)

	result := executeResult{
		dest: httpConfigFile,
		data: helpers.MustExecuteTemplate(splitClientsTemplate, splitClients),
	}

	return []executeResult{result}
}

func createSplitClients(backendGroups []dataplane.BackendGroup) []http.SplitClient {
	numSplits := 0
	for _, group := range backendGroups {
		if backendGroupNeedsSplit(group) {
			numSplits++
		}
	}

	if numSplits == 0 {
		return nil
	}

	splitClients := make([]http.SplitClient, 0, numSplits)

	for _, group := range backendGroups {
		distributions := createSplitClientDistributions(group)
		if distributions == nil {
			continue
		}

		splitClients = append(splitClients, http.SplitClient{
			VariableName:  convertStringToSafeVariableName(group.Name()),
			Distributions: distributions,
		})
	}

	return splitClients
}

func createSplitClientDistributions(group dataplane.BackendGroup) []http.SplitClientDistribution {
	if !backendGroupNeedsSplit(group) {
		return nil
	}

	backends := group.Backends

	totalWeight := int32(0)
	for _, b := range backends {
		totalWeight += b.Weight
	}

	if totalWeight == 0 {
		return []http.SplitClientDistribution{
			{
				Percent: "100",
				Value:   invalidBackendRef,
			},
		}
	}

	distributions := make([]http.SplitClientDistribution, 0, len(backends))

	// The percentage of all backends cannot exceed 100.
	availablePercentage := float64(100)

	// Iterate over all backends except the last one.
	// The last backend will get the remaining percentage.
	for i := range len(backends) - 1 {
		b := backends[i]

		percentage := percentOf(b.Weight, totalWeight)
		availablePercentage -= percentage

		distributions = append(distributions, http.SplitClientDistribution{
			Percent: fmt.Sprintf("%.2f", percentage),
			Value:   getSplitClientValue(b),
		})
	}

	// The last backend gets the remaining percentage.
	// This is done to guarantee that the sum of all percentages is 100.
	lastBackend := backends[len(backends)-1]

	distributions = append(distributions, http.SplitClientDistribution{
		Percent: fmt.Sprintf("%.2f", availablePercentage),
		Value:   getSplitClientValue(lastBackend),
	})

	return distributions
}

func getSplitClientValue(b dataplane.Backend) string {
	if b.Valid {
		return b.UpstreamName
	}
	return invalidBackendRef
}

// percentOf returns the percentage of a weight out of a totalWeight.
// The percentage is rounded to 2 decimal places using the Floor method.
// Floor is used here in order to guarantee that the sum of all percentages does not exceed 100.
// Ex. percentOf(2, 3) = 66.66
// Ex. percentOf(800, 2000) = 40.00.
func percentOf(weight, totalWeight int32) float64 {
	p := (float64(weight) * 100) / float64(totalWeight)
	return math.Floor(p*100) / 100
}

func backendGroupNeedsSplit(group dataplane.BackendGroup) bool {
	return len(group.Backends) > 1
}

// backendGroupName returns the name of the backend group.
// If the group needs to be split, the name returned is the group name.
// If the group doesn't need to be split, the name returned is the name of the backend if it is valid.
// If the name cannot be determined, it returns the name of the invalid backend upstream.
func backendGroupName(group dataplane.BackendGroup) string {
	switch len(group.Backends) {
	case 0:
		return invalidBackendRef
	case 1:
		b := group.Backends[0]
		if b.Weight == 0 || !b.Valid {
			return invalidBackendRef
		}
		return b.UpstreamName
	default:
		return group.Name()
	}
}
