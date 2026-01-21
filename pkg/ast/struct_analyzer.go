package ast

import (
	"api-doc-generator/internal/openapi"
	"go/ast"
	"reflect"
	"strings"
)

// StructAnalyzer extracts schema information from Go struct definitions
type StructAnalyzer struct {
	structs          map[string]*openapi.Schema // TypeName -> Schema
	structPriority   map[string]int             // TypeName -> Priority (higher is better)
	currentPackage   string                     // 当前解析的包名
	embeddedFields   map[string][]string        // StructName -> []EmbeddedTypeName
}

func NewStructAnalyzer() *StructAnalyzer {
	return &StructAnalyzer{
		structs:        make(map[string]*openapi.Schema),
		structPriority: make(map[string]int),
		embeddedFields: make(map[string][]string),
	}
}

// AnalyzeFile extracts all struct definitions from a Go file
func (sa *StructAnalyzer) AnalyzeFile(node *ast.File) {
	// 尝试获取包名
	packageName := ""
	if node.Name != nil {
		packageName = node.Name.Name
	}
	sa.AnalyzeFileWithPackage(node, packageName)
}

// AnalyzeFileWithPackage extracts all struct definitions from a Go file with package context
func (sa *StructAnalyzer) AnalyzeFileWithPackage(node *ast.File, packageName string) {
	sa.currentPackage = packageName
	
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

		typeName := typeSpec.Name.Name
		schema := sa.extractStructSchemaWithName(structType, typeName)
		if schema != nil {
			// 计算优先级：dao/model > service > controller
			priority := calculatePriority(packageName)
			
			// 只有优先级更高或相同时才覆盖
			if existingPriority, exists := sa.structPriority[typeName]; !exists || priority >= existingPriority {
				sa.structs[typeName] = schema
				sa.structPriority[typeName] = priority
			}
		}

		return true
	})
}

// calculatePriority 计算包的优先级
func calculatePriority(packageName string) int {
	// dao/model 层优先级最高
	if packageName == "model" || packageName == "picture" || packageName == "dao" || 
	   strings.Contains(packageName, "dao") || strings.Contains(packageName, "model") {
		return 100
	}
	// service 层其次
	if packageName == "service" || strings.Contains(packageName, "service") {
		return 50
	}
	// controller 层最低
	return 10
}

// GetSchema returns the schema for a given type name
func (sa *StructAnalyzer) GetSchema(typeName string) *openapi.Schema {
	return sa.structs[typeName]
}

// GetAllSchemas returns all extracted schemas
func (sa *StructAnalyzer) GetAllSchemas() map[string]*openapi.Schema {
	return sa.structs
}

// ExpandEmbeddedFields 展开所有嵌入字段
func (sa *StructAnalyzer) ExpandEmbeddedFields() {
	// 对每个有嵌入字段的结构体进行展开
	for structName, embeddedTypes := range sa.embeddedFields {
		schema, exists := sa.structs[structName]
		if !exists {
			continue
		}
		
		// 展开每个嵌入的类型
		for _, embeddedType := range embeddedTypes {
			embeddedSchema, exists := sa.structs[embeddedType]
			if !exists {
				continue
			}
			
			// 合并属性
			if embeddedSchema.Properties != nil {
				for propName, propSchema := range embeddedSchema.Properties {
					// 不覆盖已存在的字段
					if _, exists := schema.Properties[propName]; !exists {
						schema.Properties[propName] = propSchema
					}
				}
			}
			
			// 合并必填字段
			if embeddedSchema.Required != nil {
				if schema.Required == nil {
					schema.Required = []string{}
				}
				// 添加嵌入类型的必填字段（避免重复）
				for _, req := range embeddedSchema.Required {
					found := false
					for _, existing := range schema.Required {
						if existing == req {
							found = true
							break
						}
					}
					if !found {
						schema.Required = append(schema.Required, req)
					}
				}
			}
		}
	}
}

// extractStructSchemaWithName converts a Go struct to an OpenAPI schema with struct name context
func (sa *StructAnalyzer) extractStructSchemaWithName(structType *ast.StructType, structName string) *openapi.Schema {
	schema := &openapi.Schema{
		Type:       "object",
		Properties: make(map[string]openapi.Schema),
	}

	if structType.Fields == nil {
		return schema
	}

	for _, field := range structType.Fields.List {
		// Handle embedded fields (anonymous fields)
		if len(field.Names) == 0 {
			// This is an embedded field, record it for later expansion
			embeddedTypeName := extractTypeNameFromExpr(field.Type)
			if embeddedTypeName != "" {
				sa.embeddedFields[structName] = append(sa.embeddedFields[structName], embeddedTypeName)
			}
			continue
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

		// Extract field comment/description
		if field.Comment != nil && len(field.Comment.List) > 0 {
			// Combine all comment lines
			var comments []string
			for _, c := range field.Comment.List {
				text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
				if text != "" {
					comments = append(comments, text)
				}
			}
			if len(comments) > 0 {
				fieldSchema.Description = strings.Join(comments, " ")
			}
		}
		
		// Also try doc comments (for fields with doc above them)
		if field.Doc != nil && len(field.Doc.List) > 0 && fieldSchema.Description == "" {
			var comments []string
			for _, c := range field.Doc.List {
				text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
				if text != "" {
					comments = append(comments, text)
				}
			}
			if len(comments) > 0 {
				fieldSchema.Description = strings.Join(comments, " ")
			}
		}

		// Handle validation tags
		sa.applyValidationTags(field.Tag, &fieldSchema)
		
		// Extract gorm tag for additional description
		if field.Tag != nil {
			gormTag := reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("gorm")
			if gormTag != "" {
				// Extract comment from gorm tag
				if strings.Contains(gormTag, "comment:") {
					parts := strings.Split(gormTag, "comment:")
					if len(parts) > 1 {
						comment := strings.Split(parts[1], ";")[0]
						comment = strings.Trim(comment, `"'`)
						if comment != "" && fieldSchema.Description == "" {
							fieldSchema.Description = comment
						}
					}
				}
			}
		}

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

// extractTypeNameFromExpr extracts the type name from an expression
func extractTypeNameFromExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		// For package.Type, return just the Type name
		return t.Sel.Name
	case *ast.StarExpr:
		// For pointer types, unwrap
		return extractTypeNameFromExpr(t.X)
	}
	return ""
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
