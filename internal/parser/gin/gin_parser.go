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

	// Walk through Go files
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

		// Extract routes using AST analysis
		routes := ast.ExtractGinRoutes(node)
		for _, route := range routes {
			spec.AddPath(route.Path, route.Method, route.ToOperation())
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return spec, nil
}
