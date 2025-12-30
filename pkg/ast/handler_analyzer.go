package ast

import (
	"go/ast"
	"go/token"
	"strings"
)

// HandlerInfo contains information about handler functions
type HandlerInfo struct {
	Name         string
	RequestType  string
	ResponseType string
	QueryParams  []string
	PathParams   []string
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
		case "ShouldBindJSON", "BindJSON", "ShouldBind":
			// Extract request type from c.ShouldBindJSON(&req)
			if len(call.Args) > 0 {
				info.RequestType = extractTypeFromUnaryExpr(call.Args[0])
			}

		case "JSON":
			// Extract response type from c.JSON(200, response)
			if len(call.Args) > 1 {
				info.ResponseType = extractTypeFromExpr(call.Args[1])
			}

		case "Query":
			// Extract query parameter from c.Query("name")
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					paramName := strings.Trim(lit.Value, `"`)
					info.QueryParams = append(info.QueryParams, paramName)
				}
			}

		case "Param":
			// Extract path parameter from c.Param("id")
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					paramName := strings.Trim(lit.Value, `"`)
					info.PathParams = append(info.PathParams, paramName)
				}
			}
		}

		return true
	})
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
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.CompositeLit:
		return extractTypeFromExpr(t.Type)
	case *ast.UnaryExpr:
		return extractTypeFromUnaryExpr(t)
	}

	return ""
}
