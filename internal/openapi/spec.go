package openapi

// Spec represents an OpenAPI 3.0 specification
type Spec struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Paths      map[string]PathItem `json:"paths"`
	Components *Components         `json:"components,omitempty"`
}

type Info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

type Components struct {
	Schemas map[string]Schema `json:"schemas,omitempty"`
}

type PathItem struct {
	Get     *Operation `json:"get,omitempty"`
	Post    *Operation `json:"post,omitempty"`
	Put     *Operation `json:"put,omitempty"`
	Delete  *Operation `json:"delete,omitempty"`
	Patch   *Operation `json:"patch,omitempty"`
	Head    *Operation `json:"head,omitempty"`
	Options *Operation `json:"options,omitempty"`
}

type Operation struct {
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"` // path, query, header, cookie
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Schema      Schema `json:"schema"`
}

type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required"`
	Content     map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema Schema `json:"schema"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

type Schema struct {
	Type                 string            `json:"type,omitempty"`
	Format               string            `json:"format,omitempty"`
	Description          string            `json:"description,omitempty"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Required             []string          `json:"required,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	Ref                  string            `json:"$ref,omitempty"`
	AdditionalProperties *Schema           `json:"additionalProperties,omitempty"`
}

// NewSpec creates a new OpenAPI specification
func NewSpec() *Spec {
	return &Spec{
		OpenAPI: "3.0.0",
		Info: Info{
			Title:   "API",
			Version: "1.0.0",
		},
		Paths: make(map[string]PathItem),
		Components: &Components{
			Schemas: make(map[string]Schema),
		},
	}
}

// AddPath adds or updates a path in the specification
func (s *Spec) AddPath(path, method string, operation *Operation) {
	pathItem, exists := s.Paths[path]
	if !exists {
		pathItem = PathItem{}
	}

	switch method {
	case "GET":
		pathItem.Get = operation
	case "POST":
		pathItem.Post = operation
	case "PUT":
		pathItem.Put = operation
	case "DELETE":
		pathItem.Delete = operation
	case "PATCH":
		pathItem.Patch = operation
	case "HEAD":
		pathItem.Head = operation
	case "OPTIONS":
		pathItem.Options = operation
	}

	s.Paths[path] = pathItem
}

// AddSchema adds a schema definition to the components section
func (s *Spec) AddSchema(name string, schema Schema) {
	if s.Components == nil {
		s.Components = &Components{
			Schemas: make(map[string]Schema),
		}
	}
	s.Components.Schemas[name] = schema
}
