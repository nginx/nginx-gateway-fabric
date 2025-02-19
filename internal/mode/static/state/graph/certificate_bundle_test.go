package graph

import (
	"encoding/base64"
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidateCA(t *testing.T) {
	t.Parallel()
	base64Data := make([]byte, base64.StdEncoding.EncodedLen(len(caBlock)))
	base64.StdEncoding.Encode(base64Data, []byte(caBlock))

	tests := []struct {
		name          string
		data          []byte
		errorExpected bool
	}{
		{
			name:          "valid base64",
			data:          base64Data,
			errorExpected: false,
		},
		{
			name:          "valid plain text",
			data:          []byte(caBlock),
			errorExpected: false,
		},
		{
			name:          "invalid pem",
			data:          []byte("invalid"),
			errorExpected: true,
		},
		{
			name:          "invalid type",
			data:          []byte(caBlockInvalidType),
			errorExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			err := validateCA(test.data)
			if test.errorExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
