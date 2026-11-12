package config

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestConvertStringToSafeVariableName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg      string
		s        string
		expected string
	}{
		{
			msg:      "no hyphens",
			s:        "foo",
			expected: "foo",
		},
		{
			msg:      "hyphens",
			s:        "foo-bar-baz",
			expected: "foo_Hbar_Hbaz",
		},
		{
			msg:      "dots",
			s:        "group_ns__my.route-v1_rule0",
			expected: "group_ns__my_Droute_Hv1_rule0",
		},
		{
			msg:      "no collision between hyphen and dot",
			s:        "a-b",
			expected: "a_Hb",
		},
		{
			msg:      "dot produces different result than hyphen",
			s:        "a.b",
			expected: "a_Db",
		},
	}
	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(convertStringToSafeVariableName(test.s)).To(Equal(test.expected))
		})
	}
}

// maxVariablesHashBucketSize is the variables_hash_bucket_size configured in
// nginx.conf and nginx-plus.conf.  Generated variable names must not exceed
// this value; otherwise NGINX will reject the configuration and block reloads.
const maxVariablesHashBucketSize = 1024

func TestInferenceSplitVariableNameMaxLength(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Build worst-case Kubernetes names: alternating "a-" patterns produce
	// the maximum expansion because every hyphen becomes "_H" (+1 char each).
	// A 63-char namespace: "a-a-a-...-a" = 32 letters + 31 hyphens.
	buildAlternating := func(length int) string {
		var b strings.Builder
		b.Grow(length)
		for i := range length {
			if i%2 == 0 {
				b.WriteByte('a')
			} else {
				b.WriteByte('-')
			}
		}
		return b.String()
	}

	namespace := buildAlternating(63)  // max namespace length
	routeName := buildAlternating(253) // max HTTPRoute name length

	// Reproduce the full variable name pipeline:
	//   BackendGroup.Name() → convertStringToSafeVariableName → createInferenceSplitClientsVariableName
	groupName := "group_" + namespace + "__" + routeName + "_rule0_pathRule0"
	safeName := convertStringToSafeVariableName(groupName)
	varName := createInferenceSplitClientsVariableName(safeName)

	g.Expect(len(varName)).To(BeNumerically("<=", maxVariablesHashBucketSize),
		"inference split variable name (%d chars) exceeds variables_hash_bucket_size (%d): %s",
		len(varName), maxVariablesHashBucketSize, varName)

	// Verify the expansion is as large as expected (515 chars) so the test
	// is not silently weakened by a future change to the escaping scheme.
	g.Expect(len(varName)).To(BeNumerically(">=", 500),
		"expected the worst-case variable name to be at least 500 chars, got %d", len(varName))
}

func TestGenerateAddHeaderMapVariableName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg        string
		headerName string
		expected   string
	}{
		{
			msg:        "no hyphens",
			headerName: "MyCoolHeader",
			expected:   "mycoolheader_header_var",
		},
		{
			msg:        "with hyphens",
			headerName: "My-Cool-Header",
			expected:   "my_cool_header_header_var",
		},
	}
	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			actual := generateAddHeaderMapVariableName(test.headerName)
			g.Expect(actual).To(Equal(test.expected))
		})
	}
}
