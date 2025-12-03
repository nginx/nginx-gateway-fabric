package controller

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestCreateNginxResourceName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prefix   string
		suffix   string
		expected string
		msg      string
	}{
		{
			prefix:   "shortprefix",
			suffix:   "shortsuffix",
			expected: "shortprefix-shortsuffix",
			msg:      "short names",
		},
		{
			prefix:   strings.Repeat("a", 64),
			suffix:   "suffix",
			expected: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-b9a00a3c-suffix",
			msg:      "prefix is longer than max",
		},
		{
			prefix:   strings.Repeat("b", 60),
			suffix:   "suffix",
			expected: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-1930ffb3-suffix",
			msg:      "prefix + suffix is longer than max",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			name := CreateNginxResourceName(test.prefix, test.suffix)
			g.Expect(len(name)).To(BeNumerically("<=", maxServiceNameLen))
			g.Expect(name).To(Equal(test.expected), "expected %q, got %q", test.expected, name)
		})
	}
}

func TestCreateInferencePoolServiceName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		msg      string
	}{
		{
			name:     "pool",
			expected: "pool-pool-svc",
			msg:      "short name",
		},
		{
			name:     strings.Repeat("a", 64),
			expected: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-b33d7b99-pool-svc",
			msg:      "prefix is longer than max",
		},
		{
			name:     strings.Repeat("b", 60),
			expected: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-af3f976c-pool-svc",
			msg:      "prefix + suffix is longer than max",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			serviceName := CreateInferencePoolServiceName(test.name)
			g.Expect(len(serviceName)).To(BeNumerically("<=", maxServiceNameLen))
			g.Expect(serviceName).To(Equal(test.expected), "expected %q, got %q", test.expected, serviceName)
		})
	}
}
