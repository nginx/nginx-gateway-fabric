package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

// stringValidatingValue is a string flag value with custom validation logic.
// it implements the pflag.Value interface.
type stringValidatingValue struct {
	validator func(v string) error
	value     string
}

func (v *stringValidatingValue) String() string {
	return v.value
}

func (v *stringValidatingValue) Set(param string) error {
	if err := v.validator(param); err != nil {
		return err
	}
	v.value = param
	return nil
}

func (v *stringValidatingValue) Type() string {
	return "string"
}

// stringSliceValidatingValue is a string slice flag value with custom validation logic.
// it implements the pflag.Value interface.
type stringSliceValidatingValue struct {
	validator func(v string) error
	values    []string
	changed   bool
}

func (v *stringSliceValidatingValue) String() string {
	b := &bytes.Buffer{}
	w := csv.NewWriter(b)
	err := w.Write(v.values)
	if err != nil {
		return ""
	}

	w.Flush()
	str := strings.TrimSuffix(b.String(), "\n")
	return "[" + str + "]"
}

func (v *stringSliceValidatingValue) Set(param string) error {
	if err := v.validator(param); err != nil {
		return err
	}

	stringReader := strings.NewReader(param)
	csvReader := csv.NewReader(stringReader)
	str, err := csvReader.Read()
	if err != nil {
		return err
	}

	if !v.changed {
		v.values = str
	} else {
		v.values = append(v.values, str...)
	}
	v.changed = true

	return nil
}

func (v *stringSliceValidatingValue) Type() string {
	return "stringSlice"
}

type intValidatingValue struct {
	validator func(v int) error
	value     int
}

func (v *intValidatingValue) String() string {
	return strconv.Itoa(v.value)
}

func (v *intValidatingValue) Set(param string) error {
	intVal, err := strconv.ParseInt(param, 10, 32)
	if err != nil {
		return fmt.Errorf("failed to parse int value: %w", err)
	}

	if err := v.validator(int(intVal)); err != nil {
		return err
	}

	v.value = int(intVal)
	return nil
}

func (v *intValidatingValue) Type() string {
	return "int"
}

// namespacedNameValue is a string flag value that represents a namespaced name.
// it implements the pflag.Value interface.
type namespacedNameValue struct {
	value types.NamespacedName
}

func (v *namespacedNameValue) String() string {
	if (v.value == types.NamespacedName{}) {
		// if we don't do that, the default value in the help message will be printed as "/"
		return ""
	}
	return v.value.String()
}

func (v *namespacedNameValue) Set(param string) error {
	nsname, err := parseNamespacedResourceName(param)
	if err != nil {
		return err
	}

	v.value = nsname
	return nil
}

func (v *namespacedNameValue) Type() string {
	return "string"
}
