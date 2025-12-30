package ast

import (
	"api-doc-generator/internal/openapi"
	"go/ast"
	"reflect"
	"strings"
)

// StructAnalyzer extracts schema information from Go struct definitions
type StructAnalyzer struct {
	structs map[string]*openapi.Schema // TypeName -> Schema
}

func NewStructAnalyzer() *StructAnalyzer {
	return &StructAnalyzer{
		structs: make(map[string]*openapi.Schema),
	}
}

// AnalyzeFile extracts all struct definitions from a Go file
func (sa *StructAnalyzer) AnalyzeFile(node *ast.File) {
	ast.Inspect(node, func(n ast.Node) bool {
		// Look for type declarations
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		// Only process struct types
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		schema := sa.extractStructSchema(structType)
		if schema != nil {
			sa.structs[typeSpec.Name.Name] = schema
		}

		return true
	})
}

// GetSchema returns the schema for a given type name
func (sa *StructAnalyzer) GetSchema(typeName string) *openapi.Schema {
	return sa.structs[typeName]
}

// GetAllSchemas returns all extracted schemas
func (sa *StructAnalyzer) GetAllSchemas() map[string]*openapi.Schema {
	return sa.structs
}

// extractStructSchema converts a Go struct to an OpenAPI schema
func (sa *StructAnalyzer) extractStructSchema(structType *ast.StructType) *openapi.Schema {
	schema := &openapi.Schema{
		Type:       "object",
		Properties: make(map[string]openapi.Schema),
	}

	if structType.Fields == nil {
		return schema
	}

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue // Skip embedded fields for now
		}

		fieldName := field.Names[0].Name

		// Skip unexported fields
		if !isExported(fieldName) {
			continue
		}

		// Extract JSON tag
		jsonName, omitempty, skip := parseJSONTag(field.Tag)
		if skip {
			continue
		}

		if jsonName == "" {
			jsonName = toLowerCamelCase(fieldName)
		}

		// Extract field schema
		fieldSchema := sa.extractFieldSchema(field.Type)

		// Handle validation tags
		sa.applyValidationTags(field.Tag, &fieldSchema)

		// Mark as required if not omitempty
		if !omitempty {
			if schema.Required == nil {
				schema.Required = []string{}
			}
			schema.Required = append(schema.Required, jsonName)
		}

		schema.Properties[jsonName] = fieldSchema
	}

	return schema
}

// extractFieldSchema determines the OpenAPI schema for a field type
func (sa *StructAnalyzer) extractFieldSchema(expr ast.Expr) openapi.Schema {
	switch t := expr.(type) {
	case *ast.Ident:
		// Basic types or custom types
		return sa.identToSchema(t.Name)

	case *ast.ArrayType:
		// Array or slice
		itemSchema := sa.extractFieldSchema(t.Elt)
		return openapi.Schema{
			Type:  "array",
			Items: &itemSchema,
		}

	case *ast.StarExpr:
		// Pointer type - unwrap it
		return sa.extractFieldSchema(t.X)

	case *ast.SelectorExpr:
		// Qualified type (e.g., time.Time)
		if ident, ok := t.X.(*ast.Ident); ok {
			qualifiedName := ident.Name + "." + t.Sel.Name
			return sa.qualifiedTypeToSchema(qualifiedName)
		}

	case *ast.MapType:
		// Map types
		valueSchema := sa.extractFieldSchema(t.Value)
		return openapi.Schema{
			Type:                 "object",
			AdditionalProperties: &valueSchema,
		}
	}

	// Default to generic object
	return openapi.Schema{Type: "object"}
}

// identToSchema converts Go basic types to OpenAPI types
func (sa *StructAnalyzer) identToSchema(typeName string) openapi.Schema {
	switch typeName {
	case "string":
		return openapi.Schema{Type: "string"}
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return openapi.Schema{Type: "integer"}
	case "float32", "float64":
		return openapi.Schema{Type: "number"}
	case "bool":
		return openapi.Schema{Type: "boolean"}
	default:
		// Custom type - create a reference
		if schema, exists := sa.structs[typeName]; exists {
			return *schema
		}
		// If we don't know the type, reference it
		return openapi.Schema{Ref: "#/components/schemas/" + typeName}
	}
}

// qualifiedTypeToSchema handles types like time.Time
func (sa *StructAnalyzer) qualifiedTypeToSchema(qualifiedName string) openapi.Schema {
	switch qualifiedName {
	case "time.Time":
		return openapi.Schema{
			Type:   "string",
			Format: "date-time",
		}
	default:
		return openapi.Schema{Type: "object"}
	}
}

// parseJSONTag extracts JSON field name and options from struct tag
func parseJSONTag(tag *ast.BasicLit) (name string, omitempty bool, skip bool) {
	if tag == nil {
		return "", false, false
	}

	tagStr := strings.Trim(tag.Value, "`")
	tagValue := reflect.StructTag(tagStr).Get("json")

	if tagValue == "" {
		return "", false, false
	}

	parts := strings.Split(tagValue, ",")
	name = parts[0]

	if name == "-" {
		return "", false, true
	}

	for _, option := range parts[1:] {
		if option == "omitempty" {
			omitempty = true
		}
	}

	return name, omitempty, false
}

// applyValidationTags applies Gin binding validation tags to schema
func (sa *StructAnalyzer) applyValidationTags(tag *ast.BasicLit, schema *openapi.Schema) {
	if tag == nil {
		return
	}

	tagStr := strings.Trim(tag.Value, "`")
	bindingTag := reflect.StructTag(tagStr).Get("binding")

	if bindingTag == "" {
		return
	}

	rules := strings.Split(bindingTag, ",")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)

		switch {
		case rule == "required":
			// Handled separately
		case strings.HasPrefix(rule, "min="):
			// Could add minimum constraint
		case strings.HasPrefix(rule, "max="):
			// Could add maximum constraint
		case rule == "email":
			schema.Format = "email"
		case rule == "url":
			schema.Format = "uri"
		case strings.HasPrefix(rule, "gt="):
			// Greater than - add to description
			if schema.Description == "" {
				schema.Description = "Must be greater than " + strings.TrimPrefix(rule, "gt=")
			}
		case strings.HasPrefix(rule, "gte="):
			// Greater than or equal
			if schema.Description == "" {
				schema.Description = "Must be greater than or equal to " + strings.TrimPrefix(rule, "gte=")
			}
		}
	}
}

// isExported checks if a field name is exported
func isExported(name string) bool {
	if name == "" {
		return false
	}
	firstChar := rune(name[0])
	return firstChar >= 'A' && firstChar <= 'Z'
}

// toLowerCamelCase converts PascalCase to camelCase
func toLowerCamelCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
