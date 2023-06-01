// Package helpers contains helper functions for unit tests.
package helpers

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

// Diff prints the diff between two structs.
// It is useful in testing to compare two structs when they are large. In such a case, without Diff it will be difficult
// to pinpoint the difference between the two structs.
func Diff(want, got any) string {
	r := cmp.Diff(want, got)

	if r != "" {
		return "(-want +got)\n" + r
	}
	return r
}

// GetStringPointer takes a string and returns a pointer to it.
func GetStringPointer(s string) *string {
	return &s
}

// GetIntPointer takes an int and returns a pointer to it.
func GetIntPointer(i int) *int {
	return &i
}

// GetInt32Pointer takes an int32 and returns a pointer to it.
func GetInt32Pointer(i int32) *int32 {
	return &i
}

// GetHTTPMethodPointer takes an HTTPMethod and returns a pointer to it.
func GetHTTPMethodPointer(m v1beta1.HTTPMethod) *v1beta1.HTTPMethod {
	return &m
}

// GetHeaderMatchTypePointer takes an HeaderMatchType and returns a pointer to it.
func GetHeaderMatchTypePointer(t v1beta1.HeaderMatchType) *v1beta1.HeaderMatchType {
	return &t
}

// GetQueryParamMatchTypePointer takes an QueryParamMatchType and returns a pointer to it.
func GetQueryParamMatchTypePointer(t v1beta1.QueryParamMatchType) *v1beta1.QueryParamMatchType {
	return &t
}

// GetTLSModePointer takes a TLSModeType and returns a pointer to it.
func GetTLSModePointer(t v1beta1.TLSModeType) *v1beta1.TLSModeType {
	return &t
}

// GetBoolPointer takes a bool and returns a pointer to it.
func GetBoolPointer(b bool) *bool {
	return &b
}

// GetPointer takes a value of any type and returns a pointer to it.
func GetPointer[T any](v T) *T {
	return &v
}

// PrepareTimeForFakeClient processes the time similarly to the fake client
// from sigs.k8s.io/controller-runtime/pkg/client/fake
// making it is possible to use it in tests when comparing against values returned by the fake client.
// It panics if it fails to process the time.
func PrepareTimeForFakeClient(t metav1.Time) metav1.Time {
	bytes, err := t.Marshal()
	if err != nil {
		panic(fmt.Errorf("failed to marshal time: %w", err))
	}

	err = t.Unmarshal(bytes)
	if err != nil {
		panic(fmt.Errorf("failed to unmarshal time: %w", err))
	}

	return t
}
