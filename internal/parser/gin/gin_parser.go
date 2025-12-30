package gin

import (
	"api-doc-generator/internal/openapi"
	"api-doc-generator/pkg/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type GinParser struct{}

func NewGinParser() *GinParser {
	return &GinParser{}
}

func (p *GinParser) Name() string {
	return "Gin Framework Parser"
}

func (p *GinParser) Language() string {
	return "go-gin"
}

func (p *GinParser) Analyze(projectPath string) (*openapi.Spec, error) {
	spec := openapi.NewSpec()
	spec.Info.Title = "Auto-Generated API Documentation"
	spec.Info.Description = "Generated from code analysis"
	spec.Info.Version = "1.0.0"

	// Create struct analyzer and handler info map
	structAnalyzer := ast.NewStructAnalyzer()
	handlerInfoMap := make(map[string]*ast.HandlerInfo)

	// First pass: extract all struct schemas and handler info
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files, vendor, and tool directories
		if strings.HasSuffix(path, "_test.go") ||
			strings.Contains(path, "/vendor/") ||
			strings.Contains(path, "/tools/") ||
			strings.Contains(path, "/.git/") {
			return nil
		}

		// Parse Go file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Continue on parse errors
		}

		// Analyze structs in this file
		structAnalyzer.AnalyzeFile(node)

		// Analyze handlers in this file
		handlers := ast.AnalyzeHandlers(node)
		for name, handler := range handlers {
			handlerInfoMap[name] = handler
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Add all schemas to components
	for name, schema := range structAnalyzer.GetAllSchemas() {
		spec.AddSchema(name, *schema)
	}

	// Second pass: extract routes
	err = filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files, vendor, and tool directories
		if strings.HasSuffix(path, "_test.go") ||
			strings.Contains(path, "/vendor/") ||
			strings.Contains(path, "/tools/") ||
			strings.Contains(path, "/.git/") {
			return nil
		}

		// Parse Go file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Continue on parse errors
		}

		// Extract routes using AST analysis
		routes := ast.ExtractGinRoutes(node)
		for _, route := range routes {
			// Link handler info to route
			if handlerInfo, exists := handlerInfoMap[route.Handler]; exists {
				route.RequestType = handlerInfo.RequestType
				route.ResponseType = handlerInfo.ResponseType
			}
			spec.AddPath(route.Path, route.Method, route.ToOperation())
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return spec, nil
}
