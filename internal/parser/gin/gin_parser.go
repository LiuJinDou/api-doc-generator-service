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

	// Create analyzers
	structAnalyzer := ast.NewStructAnalyzer()
	serviceAnalyzer := ast.NewServiceAnalyzer()
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

		// Analyze structs in this file with package context
		packageName := extractPackageNameFromPath(path)
		structAnalyzer.AnalyzeFileWithPackage(node, packageName)

		// Analyze service functions (from service layer)
		if strings.Contains(path, "/service/") {
			// Extract package name from path
			parts := strings.Split(path, "/")
			for i, part := range parts {
				if part == "service" && i+1 < len(parts) {
					pkgName := parts[i+1]
					serviceAnalyzer.AnalyzeFile(node, pkgName)
					break
				}
			}
		}

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

	// Post-process: expand embedded fields
	structAnalyzer.ExpandEmbeddedFields()
	
	// Add all schemas to components
	for name, schema := range structAnalyzer.GetAllSchemas() {
		spec.AddSchema(name, *schema)
	}
	
	// Add common response wrapper schema
	spec.AddSchema("ApiResponse", openapi.Schema{
		Type: "object",
		Properties: map[string]openapi.Schema{
			"code": {
				Type:        "integer",
				Description: "响应状态码",
			},
			"message": {
				Type:        "string",
				Description: "响应消息",
			},
			"data": {
				Type:        "object",
				Description: "响应数据",
			},
		},
	})

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
				
				// Try to infer response type from service calls
				inferredType := inferResponseTypeFromServiceCalls(handlerInfo, serviceAnalyzer)
				if inferredType != "" {
					route.ResponseType = inferredType
				}
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

// extractPackageNameFromPath 从文件路径提取包名
func extractPackageNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && !strings.HasSuffix(parts[i], ".go") {
			return parts[i]
		}
	}
	return ""
}

// inferResponseTypeFromServiceCalls 从 service 调用推断响应类型
func inferResponseTypeFromServiceCalls(handlerInfo *ast.HandlerInfo, serviceAnalyzer *ast.ServiceAnalyzer) string {
	// 查找赋值给 data 变量的 service 调用
	for _, call := range handlerInfo.ServiceCalls {
		// 只处理赋值给 data 的调用
		if call.Variable != "data" {
			continue
		}
		
		// 获取 service 函数信息
		serviceInfo := serviceAnalyzer.GetServiceFuncInfo(call.Package, call.Function)
		if serviceInfo != nil {
			// 优先使用推断的数据类型
			if serviceInfo.DataType != "" {
				// 排除 map[string]interface{} 这种无法具体化的类型
				if serviceInfo.DataType != "map[string]interface{}" {
					return serviceInfo.DataType
				}
			}
			// 其次使用返回值类型
			if serviceInfo.ReturnType != "" && serviceInfo.ReturnType != "interface{}" {
				return serviceInfo.ReturnType
			}
		}
	}
	
	// 如果推断失败，返回空（使用默认 object）
	return ""
}
