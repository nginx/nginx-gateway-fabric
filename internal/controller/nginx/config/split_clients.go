package config

import (
	"fmt"
	"math"
	"strings"
	gotemplate "text/template"

	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var splitClientsTemplate = gotemplate.Must(gotemplate.New("split_clients").Parse(splitClientsTemplateText))

func executeSplitClients(conf dataplane.Configuration) []executeResult {
	splitClients := collectAllSplitClients(conf)

	result := executeResult{
		dest: httpConfigFile,
		data: helpers.MustExecuteTemplate(splitClientsTemplate, splitClients),
	}

	return []executeResult{result}
}

func collectAllSplitClients(conf dataplane.Configuration) []http.SplitClient {
	var splitClients []http.SplitClient

	splitClients = append(splitClients, createBackendGroupSplitClients(conf.BackendGroups)...)
	splitClients = append(splitClients, createRequestMirrorSplitClients(conf.HTTPServers)...)
	splitClients = append(splitClients, createRequestMirrorSplitClients(conf.SSLServers)...)
	splitClients = removeDuplicateSplitClients(splitClients)

	return splitClients
}

func createRequestMirrorSplitClients(servers []dataplane.VirtualServer) []http.SplitClient {
	var splitClients []http.SplitClient

	for _, server := range servers {
		mirrorPathToPercentage := extractMirrorTargetsWithPercentages(server.PathRules)

		for path, percentage := range mirrorPathToPercentage {
			if percentage != nil && *percentage != 100 {
				splitClient := http.SplitClient{
					// this has to be something unique and able to be accessed from the server block
					VariableName: convertSplitClientVariableName(fmt.Sprintf("%s_%.2f", path, *percentage)),
					Distributions: []http.SplitClientDistribution{
						{
							Percent: fmt.Sprintf("%.2f", *percentage),
							Value:   path,
						},
						{
							Percent: "*",
							Value:   "\"\"",
						},
					},
				}

				splitClients = append(splitClients, splitClient)
			}
		}
	}

	return splitClients
}

// convertSplitClientVariableName converts a name to a safe variable name for split clients. This includes
// replacing hypens, slashes, and dots with underscores.
func convertSplitClientVariableName(name string) string {
	safeName := convertStringToSafeVariableName(name)
	safeName = strings.ReplaceAll(safeName, "/", "_")
	safeName = strings.ReplaceAll(safeName, ".", "_")

	return safeName
}

func removeDuplicateSplitClients(splitClients []http.SplitClient) []http.SplitClient {
	seen := make(map[string]struct{})
	result := make([]http.SplitClient, 0, len(splitClients))

	for _, client := range splitClients {
		if _, exists := seen[client.VariableName]; !exists {
			seen[client.VariableName] = struct{}{}
			result = append(result, client)
		}
	}

	return result
}

func createBackendGroupSplitClients(backendGroups []dataplane.BackendGroup) []http.SplitClient {
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
		variableName := convertStringToSafeVariableName(group.Name())

		distributions := createBackendGroupSplitClientDistributions(group)
		if distributions == nil {
			continue
		}

		if group.Backends[0].EndpointPickerConfig != nil {
			// This is an inferencePool backend group, need to adjust the name.
			variableName = createInferenceSplitClientsVariableName(variableName)
		}

		splitClients = append(splitClients, http.SplitClient{
			VariableName:  variableName,
			Distributions: distributions,
		})
	}

	return splitClients
}

func createInferenceSplitClientsVariableName(groupName string) string {
	return "inference_backend_" + groupName
}

func createBackendGroupSplitClientDistributions(group dataplane.BackendGroup) []http.SplitClientDistribution {
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
			Value:   getSplitClientValue(b, group.Source, group.RuleIdx),
		})
	}

	// The last backend gets the remaining percentage.
	// This is done to guarantee that the sum of all percentages is 100.
	lastBackend := backends[len(backends)-1]

	distributions = append(distributions, http.SplitClientDistribution{
		Percent: fmt.Sprintf("%.2f", availablePercentage),
		Value:   getSplitClientValue(lastBackend, group.Source, group.RuleIdx),
	})

	return distributions
}

func getSplitClientValue(b dataplane.Backend, source types.NamespacedName, ruleIdx int) string {
	if b.Valid {
		if b.EndpointPickerConfig != nil {
			return fmt.Sprintf(
				"%s-%s-%s-%s-rule%d",
				http.InternalRoutePathPrefix,
				b.UpstreamName,
				source.Namespace,
				source.Name,
				ruleIdx,
			)
		}

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
