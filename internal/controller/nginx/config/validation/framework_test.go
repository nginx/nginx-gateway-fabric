package validation

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

type simpleValidatorFunc[T configValue] func(v T) error

type supportedValuesValidatorFunc[T configValue] func(v T) (bool, []string)

func runValidatorTests[T configValue](t *testing.T, run func(g *WithT, v T), caseNamePrefix string, values ...T) {
	t.Helper()
	for i, v := range values {
		t.Run(fmt.Sprintf("%s_case_#%d", caseNamePrefix, i), func(t *testing.T) {
			g := NewWithT(t)
			run(g, v)
		})
	}
}

func createFailureMessage[T any](v T) string {
	return fmt.Sprintf("value: %v", v)
}

func testValidValuesForSimpleValidator[T configValue](t *testing.T, f simpleValidatorFunc[T], values ...T) {
	t.Helper()
	runValidatorTests(t, func(g *WithT, v T) {
		err := f(v)
		g.Expect(err).ToNot(HaveOccurred(), createFailureMessage(v))
	}, "valid_value", values...)
}

func testInvalidValuesForSimpleValidator[T configValue](t *testing.T, f simpleValidatorFunc[T], values ...T) {
	t.Helper()
	runValidatorTests(t, func(g *WithT, v T) {
		err := f(v)
		g.Expect(err).To(HaveOccurred(), createFailureMessage(v))
	}, "invalid_value", values...)
}

func testValidValuesForSupportedValuesValidator[T configValue](
	t *testing.T,
	f supportedValuesValidatorFunc[T],
	values ...T,
) {
	t.Helper()
	runValidatorTests(
		t,
		func(g *WithT, v T) {
			valid, supportedValues := f(v)
			g.Expect(valid).To(BeTrue(), createFailureMessage(v))
			g.Expect(supportedValues).To(BeNil(), createFailureMessage(v))
		},
		"valid_value",
		values...,
	)
}

func testInvalidValuesForSupportedValuesValidator[T configValue](
	t *testing.T,
	f supportedValuesValidatorFunc[T],
	supportedValuesMap map[T]struct{},
	values ...T,
) {
	t.Helper()
	runValidatorTests(
		t,
		func(g *WithT, v T) {
			valid, supportedValues := f(v)
			g.Expect(valid).To(BeFalse(), createFailureMessage(v))
			g.Expect(supportedValues).To(Equal(getSortedKeysAsString(supportedValuesMap)), createFailureMessage(v))
		},
		"invalid_value",
		values...,
	)
}

func TestValidateInSupportedValues(t *testing.T) {
	t.Parallel()
	supportedValues := map[string]struct{}{
		"value1": {},
		"value2": {},
		"value3": {},
	}

	validator := func(value string) (bool, []string) {
		return validateInSupportedValues(value, supportedValues)
	}

	testValidValuesForSupportedValuesValidator(
		t,
		validator,
		"value1",
		"value2",
		"value3",
	)
	testInvalidValuesForSupportedValuesValidator(
		t,
		validator,
		supportedValues,
		"value4",
	)
}

func TestValidateNoUnsupportedValues(t *testing.T) {
	t.Parallel()
	unsupportedValues := map[string]struct{}{
		"badvalue1": {},
		"badvalue2": {},
		"badvalue3": {},
	}

	validator := func(value string) (bool, []string) {
		return validateNoUnsupportedValues(value, unsupportedValues)
	}

	testValidValuesForSupportedValuesValidator(
		t,
		validator,
		"value1",
		"value2",
		"value3",
	)
	testInvalidValuesForSupportedValuesValidator(
		t,
		validator,
		unsupportedValues,
		"badvalue1",
		"badvalue2",
		"badvalue3",
	)
}

func TestGetSortedKeysAsString(t *testing.T) {
	t.Parallel()
	values := map[string]struct{}{
		"value3": {},
		"value1": {},
		"value2": {},
	}

	expected := []string{"value1", "value2", "value3"}

	g := NewWithT(t)

	result := getSortedKeysAsString(values)
	g.Expect(result).To(Equal(expected))
}
