package validation

import (
	"fmt"
	"sort"
)

type configValue interface {
	int | int32 | string
}

func validateInSupportedValues[T configValue](
	value T,
	supportedValues map[T]struct{},
) (valid bool, supportedValuesAsStrings []string) {
	if _, exist := supportedValues[value]; exist {
		return true, nil
	}

	return false, getSortedKeysAsString(supportedValues)
}

func validateNoUnsupportedValues[T configValue](
	value T,
	unsupportedValues map[T]struct{},
) (valid bool, unsupportedValuesAsStrings []string) {
	if _, exist := unsupportedValues[value]; exist {
		return false, getSortedKeysAsString(unsupportedValues)
	}
	return true, nil
}

func getSortedKeysAsString[T configValue](m map[T]struct{}) []string {
	keysAsString := make([]string, 0, len(m))

	for k := range m {
		keysAsString = append(keysAsString, fmt.Sprint(k))
	}

	sort.Strings(keysAsString)

	return keysAsString
}
