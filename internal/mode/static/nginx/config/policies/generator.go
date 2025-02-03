package policies

import (
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
)

// Generator defines an interface for a policy to implement its appropriate generator functions.
//
//counterfeiter:generate . Generator
type Generator interface {
	// GenerateForServer generates policy configuration for the server block.
	GenerateForServer(policies []Policy, server http.Server) GenerateResultFiles
	// GenerateForLocation generates policy configuration for a normal location block.
	GenerateForLocation(policies []Policy, location http.Location) GenerateResultFiles
	// GenerateForInternalLocation generates policy configuration for an internal location block.
	GenerateForInternalLocation(policies []Policy) GenerateResultFiles
}

// GenerateResultFiles is a list of files generated for inclusion by policy generators.
type GenerateResultFiles []File

// File is the contents of the generated file.
type File struct {
	Name    string
	Content []byte
}

// CompositeGenerator contains all policy generators.
type CompositeGenerator struct {
	generators []Generator
}

// NewCompositeGenerator returns a new instance of a CompositeGenerator.
func NewCompositeGenerator(generators ...Generator) *CompositeGenerator {
	return &CompositeGenerator{generators: generators}
}

// GenerateForServer calls all policy generators for the server block.
func (g *CompositeGenerator) GenerateForServer(policies []Policy, server http.Server) GenerateResultFiles {
	var compositeResult GenerateResultFiles

	for _, generator := range g.generators {
		compositeResult = append(compositeResult, generator.GenerateForServer(policies, server)...)
	}

	return compositeResult
}

// GenerateForLocation calls all policy generators for a normal location block.
func (g *CompositeGenerator) GenerateForLocation(policies []Policy, location http.Location) GenerateResultFiles {
	var compositeResult GenerateResultFiles

	for _, generator := range g.generators {
		compositeResult = append(compositeResult, generator.GenerateForLocation(policies, location)...)
	}

	return compositeResult
}

// GenerateForInternalLocation calls all policy generators for an internal location block.
func (g *CompositeGenerator) GenerateForInternalLocation(policies []Policy) GenerateResultFiles {
	var compositeResult GenerateResultFiles

	for _, generator := range g.generators {
		compositeResult = append(compositeResult, generator.GenerateForInternalLocation(policies)...)
	}

	return compositeResult
}

// UnimplementedGenerator can be inherited by any policy generator that may not need to implement all of
// possible generations, in order to satisfy the Generator interface.
type UnimplementedGenerator struct{}

func (u UnimplementedGenerator) GenerateForServer(_ []Policy, _ http.Server) GenerateResultFiles {
	return nil
}

func (u UnimplementedGenerator) GenerateForLocation(_ []Policy, _ http.Location) GenerateResultFiles {
	return nil
}

func (u UnimplementedGenerator) GenerateForInternalLocation(_ []Policy) GenerateResultFiles {
	return nil
}
