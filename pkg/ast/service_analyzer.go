package ast

import (
	"go/ast"
	"strings"
)

// ServiceFuncInfo 存储 service 函数的信息
type ServiceFuncInfo struct {
	Package    string   // 包名
	Name       string   // 函数名
	ReturnType string   // 返回值类型
	DataType   string   // 实际数据类型（通过分析函数体推断）
}

// ServiceAnalyzer 分析 service 层函数
type ServiceAnalyzer struct {
	functions map[string]*ServiceFuncInfo // "package.FuncName" -> FuncInfo
}

func NewServiceAnalyzer() *ServiceAnalyzer {
	return &ServiceAnalyzer{
		functions: make(map[string]*ServiceFuncInfo),
	}
}

// AnalyzeFile 分析文件中的 service 函数
func (sa *ServiceAnalyzer) AnalyzeFile(node *ast.File, packageName string) {
	ast.Inspect(node, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// 只分析导出的函数（首字母大写）
		if !isExported(funcDecl.Name.Name) {
			return true
		}

		// 分析函数签名和函数体
		info := sa.analyzServiceFunc(funcDecl, packageName)
		if info != nil {
			key := packageName + "." + funcDecl.Name.Name
			sa.functions[key] = info
		}

		return true
	})
}

// analyzServiceFunc 分析单个 service 函数
func (sa *ServiceAnalyzer) analyzServiceFunc(funcDecl *ast.FuncDecl, packageName string) *ServiceFuncInfo {
	info := &ServiceFuncInfo{
		Package: packageName,
		Name:    funcDecl.Name.Name,
	}

	// 提取返回值类型
	if funcDecl.Type != nil && funcDecl.Type.Results != nil {
		for _, field := range funcDecl.Type.Results.List {
			typeName := extractTypeNameFromExpr(field.Type)
			if typeName != "" && typeName != "error" {
				info.ReturnType = typeName
				break
			}
		}
	}

	// 分析函数体，推断实际数据类型
	if funcDecl.Body != nil {
		dataType := sa.inferDataTypeFromBody(funcDecl.Body)
		if dataType != "" {
			info.DataType = dataType
		}
	}

	return info
}

// inferDataTypeFromBody 从函数体推断实际数据类型
func (sa *ServiceAnalyzer) inferDataTypeFromBody(body *ast.BlockStmt) string {
	var inferredType string
	var lastDataType string // 记录最后一次 data 变量的类型

	// 查找变量声明和赋值
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// 查找 data 变量的赋值
			for i, lhs := range node.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "data" && i < len(node.Rhs) {
					// 尝试从右侧提取类型
					typeName := extractTypeFromAssignment(node.Rhs[i])
					if typeName != "" {
						lastDataType = typeName
						// 如果不是 map[string]interface{}，立即返回
						if typeName != "map[string]interface{}" {
							inferredType = typeName
						}
					}
				}
			}

		case *ast.ValueSpec:
			// 查找 var data Type 声明
			for _, name := range node.Names {
				if name.Name == "data" && node.Type != nil {
					typeName := extractDetailedTypeName(node.Type)
					if typeName != "" {
						lastDataType = typeName
						// 如果不是 map 或 interface，立即返回
						if typeName != "map[string]interface{}" && typeName != "interface{}" {
							inferredType = typeName
						}
					}
				}
			}
		}
		return true
	})

	// 如果没有找到具体类型，使用最后一次的类型
	if inferredType == "" && lastDataType != "" {
		inferredType = lastDataType
	}

	return inferredType
}

// extractTypeFromAssignment 从赋值表达式提取类型
func extractTypeFromAssignment(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.CompositeLit:
		// data := []Type{} 或 data := Type{}
		typeName := extractTypeNameFromExpr(t.Type)
		return cleanTypeName(typeName)
		
	case *ast.CallExpr:
		// data := make([]Type, 10) 或 data := NewType()
		if ident, ok := t.Fun.(*ast.Ident); ok {
			if ident.Name == "make" && len(t.Args) > 0 {
				typeName := extractTypeNameFromExpr(t.Args[0])
				return cleanTypeName(typeName)
			}
		}
		// 对于函数调用，可能无法推断
		
	case *ast.UnaryExpr:
		// data := &Type{}
		typeName := extractTypeNameFromExpr(t.X)
		return cleanTypeName(typeName)
		
	case *ast.Ident:
		// data := someVariable
		return t.Name
	}
	return ""
}

// cleanTypeName 清理类型名称，去除包名前缀
func cleanTypeName(typeName string) string {
	// picture.PictureLing -> PictureLing
	parts := strings.Split(typeName, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return typeName
}

// extractDetailedTypeName 提取详细类型名称，包括数组、指针等
func extractDetailedTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
		
	case *ast.SelectorExpr:
		// package.Type -> Type
		return t.Sel.Name
		
	case *ast.ArrayType:
		// []Type -> Type (返回元素类型，外层会包装成数组)
		elemType := extractDetailedTypeName(t.Elt)
		return "[]" + elemType  // 保留数组标记
		
	case *ast.StarExpr:
		// *Type -> Type
		return extractDetailedTypeName(t.X)
		
	case *ast.MapType:
		// map[string]Type
		keyType := extractDetailedTypeName(t.Key)
		valueType := extractDetailedTypeName(t.Value)
		return "map[" + keyType + "]" + valueType
		
	case *ast.InterfaceType:
		return "interface{}"
	}
	
	return ""
}


// GetServiceFuncInfo 获取 service 函数信息
func (sa *ServiceAnalyzer) GetServiceFuncInfo(packageName, funcName string) *ServiceFuncInfo {
	key := packageName + "." + funcName
	return sa.functions[key]
}

// GetAllFunctions 获取所有 service 函数信息
func (sa *ServiceAnalyzer) GetAllFunctions() map[string]*ServiceFuncInfo {
	return sa.functions
}

