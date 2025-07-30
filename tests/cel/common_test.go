package cel

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestMustGenerateRandomPrimeNumer(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	g.Expect(func() {
		_ = RandomPrimeNumber()
	}).ToNot(Panic())
}

func TestMustReturnUniqueResourceName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	name := "test-resource"
	uniqueName := UniqueResourceName(name)

	g.Expect(uniqueName).To(HavePrefix(name))
	g.Expect(len(uniqueName)).To(BeNumerically(">", len(name)))
}
