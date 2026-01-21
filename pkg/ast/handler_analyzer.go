package ast

import (
	"go/ast"
	"go/token"
	"strings"
)

// HandlerInfo contains information about handler functions
type HandlerInfo struct {
	Name            string
	RequestType     string
	ResponseType    string
	QueryParams     []string
	PathParams      []string
	ServiceCalls    []ServiceCall // 记录 service 函数调用
}

// ServiceCall 记录 service 函数调用信息
type ServiceCall struct {
	Package  string // 包名
	Function string // 函数名
	Variable string // 赋值给的变量名
}

// AnalyzeHandlers extracts handler function information from a Go file
func AnalyzeHandlers(node *ast.File) map[string]*HandlerInfo {
	handlers := make(map[string]*HandlerInfo)

	ast.Inspect(node, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Check if this is a handler function (has gin.Context parameter)
		if !isGinHandler(funcDecl) {
			return true
		}

		info := &HandlerInfo{
			Name:        funcDecl.Name.Name,
			QueryParams: []string{},
			PathParams:  []string{},
		}

		// Analyze function body to find request/response types
		if funcDecl.Body != nil {
			analyzeHandlerBody(funcDecl.Body, info)
		}

		handlers[funcDecl.Name.Name] = info

		return true
	})

	return handlers
}

// isGinHandler checks if a function is a Gin handler
func isGinHandler(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Type == nil || funcDecl.Type.Params == nil {
		return false
	}

	for _, param := range funcDecl.Type.Params.List {
		// Look for *gin.Context parameter
		if starExpr, ok := param.Type.(*ast.StarExpr); ok {
			if selExpr, ok := starExpr.X.(*ast.SelectorExpr); ok {
				if ident, ok := selExpr.X.(*ast.Ident); ok {
					if ident.Name == "gin" && selExpr.Sel.Name == "Context" {
						return true
					}
				}
			}
		}
	}

	return false
}

// analyzeHandlerBody extracts request/response type information from handler body
func analyzeHandlerBody(body *ast.BlockStmt, info *HandlerInfo) {
	// Track variable types within the function
	varTypes := make(map[string]string)
	
	// First pass: collect variable declarations and assignments
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Handle var := value or var, err := func()
			for i, lhs := range node.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && i < len(node.Rhs) {
					// Try to extract type from RHS
					typeName := extractTypeFromExpr(node.Rhs[i])
					if typeName != "" {
						varTypes[ident.Name] = typeName
					}
					
					// Check for function call return types
					if call, ok := node.Rhs[i].(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if x, ok := sel.X.(*ast.Ident); ok {
								// Track service calls like: data, err := home.GetAllLabels()
								packageName := x.Name
								funcName := sel.Sel.Name
								
								// 记录 service 调用
								info.ServiceCalls = append(info.ServiceCalls, ServiceCall{
									Package:  packageName,
									Function: funcName,
									Variable: ident.Name,
								})
								
								// 用特殊标记表示这是 service 调用的返回值
								varTypes[ident.Name] = packageName + "." + funcName + ".result"
							}
						}
					}
				}
			}
			
		case *ast.ValueSpec:
			// Handle var declarations with type: var params home.CreateLabels
			if node.Type != nil {
				typeName := extractTypeFromExpr(node.Type)
				// Clean up type name: home.CreateLabels -> CreateLabels
				if strings.Contains(typeName, ".") {
					parts := strings.Split(typeName, ".")
					typeName = parts[len(parts)-1] // Use the last part (actual type name)
				}
				for _, name := range node.Names {
					varTypes[name.Name] = typeName
				}
			}
		}
		return true
	})
	
	// Second pass: analyze function calls
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		switch sel.Sel.Name {
		case "ShouldBindJSON", "BindJSON", "ShouldBind", "Bind", "ShouldBindQuery":
			// Extract request type from c.ShouldBindJSON(&req)
			if len(call.Args) > 0 && info.RequestType == "" {
				// First try to get from variable type mapping
				foundType := false
				if unary, ok := call.Args[0].(*ast.UnaryExpr); ok {
					if ident, ok := unary.X.(*ast.Ident); ok {
						if varType, exists := varTypes[ident.Name]; exists && varType != "" {
							info.RequestType = varType
							foundType = true
						}
					}
				}
				
				// Fallback: try direct type extraction
				if !foundType {
					reqType := extractTypeFromUnaryExpr(call.Args[0])
					if reqType != "" {
						info.RequestType = reqType
					}
				}
			}

		case "JSON":
			// Extract response type from c.JSON(200, response)
			if len(call.Args) > 1 {
				respType := extractTypeFromExpr(call.Args[1])
				if respType == "" {
					// Try to get from variable name
					if ident, ok := call.Args[1].(*ast.Ident); ok {
						if varType, exists := varTypes[ident.Name]; exists {
							respType = varType
						}
					}
				}
				if respType != "" && info.ResponseType == "" {
					info.ResponseType = respType
				}
			}
			
		case "SetResponseOK", "Success", "OK":
			// Handle custom response wrappers like tool.SetResponseOK(c, data)
			if len(call.Args) > 1 {
				// 先尝试从变量类型映射获取
				if ident, ok := call.Args[1].(*ast.Ident); ok {
					if varType, exists := varTypes[ident.Name]; exists {
						// 如果是 service 调用的结果标记，保持原样让后续处理
						if strings.Contains(varType, ".result") {
							info.ResponseType = ident.Name + ".service"
						} else if varType != "" {
							info.ResponseType = varType
						}
					} else {
						// 没有类型映射，使用变量名
						info.ResponseType = ident.Name
					}
				} else {
					// 不是简单标识符，尝试直接提取类型
					respType := extractTypeFromExpr(call.Args[1])
					if respType != "" {
						info.ResponseType = respType
					}
				}
			}

		case "Query", "DefaultQuery":
			// Extract query parameter from c.Query("name")
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					paramName := strings.Trim(lit.Value, `"`)
					info.QueryParams = appendUnique(info.QueryParams, paramName)
				}
			}

		case "Param":
			// Extract path parameter from c.Param("id")
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					paramName := strings.Trim(lit.Value, `"`)
					info.PathParams = appendUnique(info.PathParams, paramName)
				}
			}
		}

		return true
	})
}

// appendUnique appends a string to a slice if it doesn't already exist
func appendUnique(slice []string, item string) []string {
	for _, existing := range slice {
		if existing == item {
			return slice
		}
	}
	return append(slice, item)
}

// extractTypeFromUnaryExpr extracts type name from &Type{} or &var
func extractTypeFromUnaryExpr(expr ast.Expr) string {
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok {
		return ""
	}

	switch t := unary.X.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.CompositeLit:
		return extractTypeFromExpr(t.Type)
	}

	return ""
}

// extractTypeFromExpr extracts type name from various expression types
func extractTypeFromExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		// For package.Type, return just the Type name
		return t.Sel.Name
	case *ast.CompositeLit:
		return extractTypeFromExpr(t.Type)
	case *ast.UnaryExpr:
		return extractTypeFromUnaryExpr(t)
	case *ast.MapType:
		// For map types, return a generic name
		return "map[string]interface{}"
	}

	return ""
}
