/*
Copyright 2025 The NGINX Gateway Fabric Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package snippetspolicy

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// Validator validates a SnippetsPolicy.
// Implements policies.Validator interface.
type Validator struct{}

// NewValidator returns a new instance of Validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates the spec of a SnippetsPolicy.
func (v *Validator) Validate(policy policies.Policy) []conditions.Condition {
	sp := helpers.MustCastObject[*ngfAPI.SnippetsPolicy](policy)

	targetRefsPath := field.NewPath("spec").Child("targetRefs")
	supportedKinds := []gatewayv1.Kind{kinds.Gateway}
	supportedGroups := []gatewayv1.Group{gatewayv1.GroupName}

	// Validate TargetRef
	seenTargetRefs := make(map[string]struct{})
	for i, targetRef := range sp.Spec.TargetRefs {
		if err := policies.ValidateTargetRef(
			targetRef,
			targetRefsPath.Index(i),
			supportedGroups,
			supportedKinds,
		); err != nil {
			return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
		}

		if _, exists := seenTargetRefs[string(targetRef.Name)]; exists {
			msg := fmt.Sprintf("duplicate targetRef name %q", targetRef.Name)
			return []conditions.Condition{conditions.NewPolicyInvalid(msg)}
		}
		seenTargetRefs[string(targetRef.Name)] = struct{}{}
	}

	// Validate Snippets
	if err := validateSnippets(sp.Spec.Snippets); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	return nil
}

// ValidateGlobalSettings validates a SnippetsPolicy with respect to the NginxProxy global settings.
func (v *Validator) ValidateGlobalSettings(
	_ policies.Policy,
	_ *policies.GlobalSettings,
) []conditions.Condition {
	return nil
}

// Conflicts returns true if the two SnippetsPolicies conflict.
// SnippetsPolicies are merged by lexicographic order, so they don't inherently conflict
// in a way that prevents them from being applied together (structurally).
// Detailed logical conflicts (e.g. conflicting NGINX directives) are caught by nginx -t.
func (v *Validator) Conflicts(_, _ policies.Policy) bool {
	return false
}

func validateSnippets(snippets []ngfAPI.Snippet) error {
	seenContexts := make(map[ngfAPI.NginxContext]struct{})
	for _, snippet := range snippets {
		if _, exists := seenContexts[snippet.Context]; exists {
			return fmt.Errorf("duplicate context %q", snippet.Context)
		}
		seenContexts[snippet.Context] = struct{}{}
	}
	return nil
}
