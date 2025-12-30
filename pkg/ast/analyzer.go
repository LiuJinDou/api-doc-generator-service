package ast

import (
	"api-doc-generator/internal/openapi"
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

type RouteInfo struct {
	Method      string
	Path        string
	Handler     string
	HandlerFunc *ast.FuncDecl
	HasBody     bool
	HasParam    bool
	GroupPrefix string
	RequestType string
	ResponseType string
}

// ExtractGinRoutes extracts Gin route definitions from AST
func ExtractGinRoutes(node *ast.File) []RouteInfo {
	var routes []RouteInfo
	groupPrefixes := make(map[string]string) // Track router variable to prefix mapping

	// First pass: find route groups
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Look for r.Group("/prefix") patterns
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if sel.Sel.Name == "Group" && len(call.Args) > 0 {
				if pathLit, ok := call.Args[0].(*ast.BasicLit); ok && pathLit.Kind == token.STRING {
					prefix := strings.Trim(pathLit.Value, `"`)
					// Try to find the variable this is assigned to
					// This is a simplified approach - could be enhanced
					groupPrefixes["v1"] = prefix // Common pattern
					groupPrefixes["products"] = prefix
				}
			}
		}

		return true
	})

	// Second pass: extract routes with group context
	ast.Inspect(node, func(n ast.Node) bool {
		// Track assignment statements for route groups
		if assign, ok := n.(*ast.AssignStmt); ok {
			for i, lhs := range assign.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && i < len(assign.Rhs) {
					if call, ok := assign.Rhs[i].(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if sel.Sel.Name == "Group" && len(call.Args) > 0 {
								if pathLit, ok := call.Args[0].(*ast.BasicLit); ok {
									newPrefix := strings.Trim(pathLit.Value, `"`)

									// Check if this is a nested group (v1.Group or products.Group)
									if selIdent, ok := sel.X.(*ast.Ident); ok {
										parentPrefix := groupPrefixes[selIdent.Name]
										// Concatenate parent prefix with new prefix
										groupPrefixes[ident.Name] = parentPrefix + newPrefix
									} else {
										groupPrefixes[ident.Name] = newPrefix
									}
								}
							}
						}
					}
				}
			}
		}

		// Look for function calls that might be Gin route registrations
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		route := parseGinRouteCall(call, groupPrefixes)
		if route != nil {
			routes = append(routes, *route)
		}

		return true
	})

	return routes
}

func parseGinRouteCall(call *ast.CallExpr, groupPrefixes map[string]string) *RouteInfo {
	// Match patterns like: r.GET("/path", handler) or router.POST("/users", CreateUser)
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	method := sel.Sel.Name
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true,
		"DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true,
	}

	if !validMethods[method] {
		return nil
	}

	// Need at least path and handler
	if len(call.Args) < 2 {
		return nil
	}

	// Extract path (first argument)
	pathLit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || pathLit.Kind != token.STRING {
		return nil
	}
	path := strings.Trim(pathLit.Value, `"`)

	// Determine group prefix from the router variable
	var groupPrefix string
	if ident, ok := sel.X.(*ast.Ident); ok {
		if prefix, exists := groupPrefixes[ident.Name]; exists {
			groupPrefix = prefix
		}
	}

	// Combine group prefix with path
	fullPath := groupPrefix + path
	if fullPath == "" {
		fullPath = "/"
	}

	// Extract handler name (second argument)
	var handler string
	switch h := call.Args[1].(type) {
	case *ast.Ident:
		handler = h.Name
	case *ast.SelectorExpr:
		handler = h.Sel.Name
	default:
		handler = "handler"
	}

	return &RouteInfo{
		Method:      method,
		Path:        convertGinPathToOpenAPI(fullPath),
		Handler:     handler,
		HasBody:     method == "POST" || method == "PUT" || method == "PATCH",
		HasParam:    strings.Contains(path, ":"),
		GroupPrefix: groupPrefix,
	}
}

func convertGinPathToOpenAPI(ginPath string) string {
	// Convert Gin path params :id to OpenAPI {id}
	re := regexp.MustCompile(`:([a-zA-Z0-9_]+)`)
	return re.ReplaceAllString(ginPath, `{$1}`)
}

func (r *RouteInfo) ToOperation() *openapi.Operation {
	op := &openapi.Operation{
		Summary:   formatHandlerName(r.Handler),
		Tags:      extractTags(r.Path),
		Responses: make(map[string]openapi.Response),
	}

	// Add path parameters if present
	if r.HasParam {
		op.Parameters = extractPathParameters(r.Path)
	}

	// Add request body for methods that typically have one
	if r.HasBody {
		var requestSchema openapi.Schema
		if r.RequestType != "" {
			// Use schema reference if we know the type
			requestSchema = openapi.Schema{
				Ref: "#/components/schemas/" + r.RequestType,
			}
		} else {
			// Fallback to generic object
			requestSchema = openapi.Schema{Type: "object"}
		}

		op.RequestBody = &openapi.RequestBody{
			Required: true,
			Content: map[string]openapi.MediaType{
				"application/json": {
					Schema: requestSchema,
				},
			},
		}
	}

	// Response schema
	var responseSchema openapi.Schema
	if r.ResponseType != "" {
		// Use schema reference if we know the type
		responseSchema = openapi.Schema{
			Ref: "#/components/schemas/" + r.ResponseType,
		}
	} else {
		// Fallback to generic object
		responseSchema = openapi.Schema{Type: "object"}
	}

	// Success response
	op.Responses["200"] = openapi.Response{
		Description: "Successful response",
		Content: map[string]openapi.MediaType{
			"application/json": {
				Schema: responseSchema,
			},
		},
	}

	// Error responses
	op.Responses["400"] = openapi.Response{Description: "Bad request"}
	op.Responses["404"] = openapi.Response{Description: "Not found"}
	op.Responses["500"] = openapi.Response{Description: "Internal server error"}

	return op
}

func formatHandlerName(name string) string {
	// Convert camelCase/PascalCase to readable format
	// GetUser -> "Get User"
	// CreateUserProfile -> "Create User Profile"
	re := regexp.MustCompile(`([A-Z])`)
	spaced := re.ReplaceAllString(name, " $1")
	return strings.TrimSpace(spaced)
}

func extractTags(path string) []string {
	// Extract meaningful resource name from path
	// /api/v1/products -> ["Products"]
	// /api/v1/products/{id} -> ["Products"]
	// /health -> ["Health"]
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Skip common prefixes like api, v1, v2, etc.
	var resourcePart string
	for _, part := range parts {
		// Skip version patterns and "api" prefix
		if part == "api" || regexp.MustCompile(`^v\d+$`).MatchString(part) {
			continue
		}
		// Skip path parameters
		if !strings.HasPrefix(part, "{") {
			resourcePart = part
			break
		}
	}

	if resourcePart != "" {
		// Capitalize first letter for better presentation
		return []string{strings.Title(resourcePart)}
	}
	return []string{"Default"}
}

func extractPathParameters(path string) []openapi.Parameter {
	var params []openapi.Parameter
	re := regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)
	matches := re.FindAllStringSubmatch(path, -1)

	for _, match := range matches {
		params = append(params, openapi.Parameter{
			Name:     match[1],
			In:       "path",
			Required: true,
			Schema:   openapi.Schema{Type: "string"},
		})
	}

	return params
}
