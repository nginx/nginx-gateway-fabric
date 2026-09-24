package shared_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
)

func TestParseSize_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected int64
	}{
		{input: "256k", expected: 256 * 1024},
		{input: "1m", expected: 1024 * 1024},
		{input: "512m", expected: 512 * 1024 * 1024},
		{input: "128k", expected: 128 * 1024},
		{input: "1g", expected: 1024 * 1024 * 1024},
		{input: "2g", expected: 2 * 1024 * 1024 * 1024},
		{input: "1024", expected: 1024},
		{input: "  256k  ", expected: 256 * 1024},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result, err := shared.ParseSize(test.input)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestParseSize_Invalid(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"256x",
		"k",
		"abc256k",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			_, err := shared.ParseSize(input)
			g.Expect(err).To(HaveOccurred())
		})
	}
}
