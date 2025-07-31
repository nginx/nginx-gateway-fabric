package cel

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestMustGenerateRandomPrimeNumber(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	g.Expect(func() {
		_ = randomPrimeNumber()
	}).ToNot(Panic())
}

func TestMustReturnUniqueResourceName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	name := "test-resource"
	uniqueName := uniqueResourceName(name)

	g.Expect(uniqueName).To(HavePrefix(name))
	g.Expect(len(uniqueName)).To(BeNumerically(">", len(name)))
}
