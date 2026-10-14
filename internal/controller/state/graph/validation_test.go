package graph

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidateHostname(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		hostname    string
		errContains string
		expectErr   bool
	}{
		{
			hostname:  "example.com",
			expectErr: false,
			name:      "valid hostname",
		},
		{
			hostname:    "",
			expectErr:   true,
			errContains: "cannot be empty string",
			name:        "empty hostname",
		},
		{
			hostname:  "*.example.com",
			expectErr: false,
			name:      "wildcard hostname",
		},
		{
			hostname:    "example$com",
			expectErr:   true,
			errContains: "a lowercase RFC 1123 subdomain",
			name:        "invalid hostname",
		},
		{
			hostname:    "*.example.*.com",
			expectErr:   true,
			errContains: "a wildcard DNS-1123 subdomain",
			name:        "invalid wildcard hostname",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			err := validateHostname(test.hostname)

			if test.expectErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.errContains))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
