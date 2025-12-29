package parser

import (
	"api-doc-generator/internal/openapi"
	"errors"
)

// Parser interface - implement this for each language/framework
type Parser interface {
	// Analyze the code and return OpenAPI spec
	Analyze(projectPath string) (*openapi.Spec, error)

	// Get parser name
	Name() string

	// Get supported language/framework
	Language() string
}

// Registry manages available parsers
type Registry struct {
	parsers map[string]Parser
}

func NewRegistry() *Registry {
	return &Registry{
		parsers: make(map[string]Parser),
	}
}

func (r *Registry) Register(name string, parser Parser) {
	r.parsers[name] = parser
}

func (r *Registry) Get(name string) (Parser, error) {
	parser, ok := r.parsers[name]
	if !ok {
		return nil, errors.New("parser not found: " + name)
	}
	return parser, nil
}

func (r *Registry) List() []string {
	var names []string
	for name := range r.parsers {
		names = append(names, name)
	}
	return names
}
