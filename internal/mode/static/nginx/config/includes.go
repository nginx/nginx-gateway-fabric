package config

import (
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

// createIncludeExecuteResultsFromServers creates a list of executeResults -- or NGINX config files -- from all
// the includes in the provided servers. Since there may be duplicate includes, such as configuration for policies that
// apply to multiple routes, or snippets filters that are attached to multiple routing rules, this function deduplicates
// all includes, ensuring only a single file per unique include is generated.
func createIncludeExecuteResultsFromServers(servers []http.Server) []executeResult {
	uniqueIncludes := make(map[string][]byte)

	// deduplicate include files across servers and location
	for _, server := range servers {
		for _, include := range server.Includes {
			uniqueIncludes[include.Name] = include.Content
		}

		for _, loc := range server.Locations {
			for _, include := range loc.Includes {
				uniqueIncludes[include.Name] = include.Content
			}
		}
	}

	results := make([]executeResult, 0, len(uniqueIncludes))

	for filename, contents := range uniqueIncludes {
		results = append(results, executeResult{
			dest: filename,
			data: contents,
		})
	}

	return results
}

// createIncludesFromPolicyGenerateResult converts a list of policies.File into a list of includes.
func createIncludesFromPolicyGenerateResult(resFiles []policies.File) []shared.Include {
	if len(resFiles) == 0 {
		return nil
	}

	includes := make([]shared.Include, 0, len(resFiles))
	for _, file := range resFiles {
		includes = append(includes, shared.Include{
			Name:    includesFolder + "/" + file.Name,
			Content: file.Content,
		})
	}

	return includes
}

// createIncludeFromSnippet converts a dataplane.Snippet into an include.
func createIncludeFromSnippet(snippet dataplane.Snippet) shared.Include {
	return shared.Include{
		Name:    includesFolder + "/" + snippet.Name + ".conf",
		Content: []byte(snippet.Contents),
	}
}

// deduplicateIncludes deduplicates all the includes using the include name as the identifier.
// Duplicate includes are possible when a single policy targets multiple resources, or a snippets filter
// is referenced on multiple routing rules.
func deduplicateIncludes(includes []shared.Include) []shared.Include {
	uniqueIncludes := make(map[string]shared.Include)
	for _, i := range includes {
		if _, ok := uniqueIncludes[i.Name]; !ok {
			uniqueIncludes[i.Name] = i
		}
	}

	results := make([]shared.Include, 0, len(uniqueIncludes))
	for _, i := range uniqueIncludes {
		results = append(results, i)
	}

	return results
}

// createIncludesFromLocationSnippetsFilters creates includes for a location from a list of SnippetsFilters.
// A SnippetsFilter can have both a server snippet and a location snippet. This function converts
// all the location snippets in the SnippetsFilters to includes.
func createIncludesFromLocationSnippetsFilters(filters []dataplane.SnippetsFilter) []shared.Include {
	if len(filters) == 0 {
		return nil
	}

	includes := make([]shared.Include, 0)

	for _, f := range filters {
		if f.LocationSnippet != nil {
			includes = append(includes, createIncludeFromSnippet(*f.LocationSnippet))
		}
	}

	return deduplicateIncludes(includes)
}

// createIncludesFromServerSnippetsFilters creates includes for a server from a dataplane.VirtualServer.
// It finds all the server snippets from the SnippetsFilters on each MatchRule. This function converts all
// the server snippets into includes.
func createIncludesFromServerSnippetsFilters(server dataplane.VirtualServer) []shared.Include {
	if len(server.PathRules) == 0 {
		return nil
	}

	includes := make([]shared.Include, 0)

	for _, pr := range server.PathRules {
		for _, mr := range pr.MatchRules {
			for _, sf := range mr.Filters.SnippetsFilters {
				if sf.ServerSnippet != nil {
					includes = append(includes, createIncludeFromSnippet(*sf.ServerSnippet))
				}
			}
		}
	}

	return deduplicateIncludes(includes)
}

// createIncludesFromSnippets converts a list of Snippets to a list of Includes.
// Used for main and http snippets only. Server and location snippets are handled by other functions above.
func createIncludesFromSnippets(snippets []dataplane.Snippet) []shared.Include {
	if len(snippets) == 0 {
		return nil
	}

	includes := make([]shared.Include, 0)

	for _, s := range snippets {
		includes = append(includes, createIncludeFromSnippet(s))
	}

	return deduplicateIncludes(includes)
}

// createIncludeExecuteResults creates a list of executeResults -- or NGINX config files -- from a list of includes.
// Used for main and http snippets only. Server and location snippets are handled by other functions above.
func createIncludeExecuteResults(includes []shared.Include) []executeResult {
	results := make([]executeResult, 0, len(includes))

	for _, inc := range includes {
		results = append(results, executeResult{
			dest: inc.Name,
			data: inc.Content,
		})
	}

	return results
}
