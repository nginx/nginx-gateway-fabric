package main

import (
	"log"
	"os"
	"text/template"
)

//go:generate go run gen_policies.go

// Define policy types
var policies = []string{
	"ObservabilityPolicy",
}

// Template for generating policy methods
const tmpl = `// Code generated by gen_policies.go; DO NOT EDIT.

package v1alpha2

import "sigs.k8s.io/gateway-api/apis/v1alpha2"

{{ range . }}
func (p *{{ . }}) GetTargetRefs() []v1alpha2.LocalPolicyTargetReference {
    return p.Spec.TargetRefs
}

func (p *{{ . }}) GetPolicyStatus() v1alpha2.PolicyStatus {
    return p.Status
}

func (p *{{ . }}) SetPolicyStatus(status v1alpha2.PolicyStatus) {
    p.Status = status
}
{{ end }}
`

func main() {
	// Ensure the output directory exists
	outputDir := "../apis/v1alpha2"
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		log.Fatalf("failed to create directory: %v", err)
	}

	// Open the output file
	file, err := os.Create("../policy_methods.go")
	if err != nil {
		log.Fatalf("failed to create file: %v", err)
	}
	defer file.Close()

	// Create a template and execute it with the policy list
	t := template.Must(template.New("policy").Parse(tmpl))
	if err := t.Execute(file, policies); err != nil {
		log.Fatalf("failed to execute template: %v", err)
	}

	log.Printf("Writing PolicyMethods to %s... Done\n", "policy_methods.go")
}
