package graph

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidateHostname(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		hostname  string
		expectErr bool
	}{
		{
			hostname:  "example.com",
			expectErr: false,
			name:      "valid hostname",
		},
		{
			hostname:  "",
			expectErr: true,
			name:      "empty hostname",
		},
		{
			hostname:  "*.example.com",
			expectErr: false,
			name:      "wildcard hostname",
		},
		{
			hostname:  "example$com",
			expectErr: true,
			name:      "invalid hostname",
		},
		{
			hostname:  "*.example.*.com",
			expectErr: true,
			name:      "invalid wildcard hostname",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			err := validateHostname(test.hostname)

			if test.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
