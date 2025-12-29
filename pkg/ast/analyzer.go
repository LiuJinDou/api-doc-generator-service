package ast

import (
	"api-doc-generator/internal/openapi"
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

type RouteInfo struct {
	Method   string
	Path     string
	Handler  string
	HasBody  bool
	HasParam bool
}

// ExtractGinRoutes extracts Gin route definitions from AST
func ExtractGinRoutes(node *ast.File) []RouteInfo {
	var routes []RouteInfo

	ast.Inspect(node, func(n ast.Node) bool {
		// Look for function calls that might be Gin route registrations
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		route := parseGinRouteCall(call)
		if route != nil {
			routes = append(routes, *route)
		}

		return true
	})

	return routes
}

func parseGinRouteCall(call *ast.CallExpr) *RouteInfo {
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
		Method:   method,
		Path:     convertGinPathToOpenAPI(path),
		Handler:  handler,
		HasBody:  method == "POST" || method == "PUT" || method == "PATCH",
		HasParam: strings.Contains(path, ":"),
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
		op.RequestBody = &openapi.RequestBody{
			Required: true,
			Content: map[string]openapi.MediaType{
				"application/json": {
					Schema: openapi.Schema{Type: "object"},
				},
			},
		}
	}

	// Default responses
	op.Responses["200"] = openapi.Response{
		Description: "Successful response",
		Content: map[string]openapi.MediaType{
			"application/json": {
				Schema: openapi.Schema{Type: "object"},
			},
		},
	}
	op.Responses["400"] = openapi.Response{Description: "Bad request"}
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
	// Extract first path segment as tag
	// /users/123 -> ["users"]
	// /api/v1/products -> ["api"]
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		return []string{parts[0]}
	}
	return []string{"default"}
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
