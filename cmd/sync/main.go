package main

import (
	"api-doc-generator/internal/config"
	"api-doc-generator/internal/openapi"
	"api-doc-generator/internal/parser/gin"
	"api-doc-generator/internal/sync"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	// 定义命令行参数
	projectName := flag.String("project", "", "项目名称（配置文件名，不含.json后缀）")
	configDir := flag.String("config-dir", ".temp/configs", "配置文件目录")
	listProjects := flag.Bool("list", false, "列出所有可用的项目")
	showInfo := flag.Bool("info", false, "显示项目详细信息")
	saveOutput := flag.Bool("save", false, "保存 OpenAPI 规范到文件")
	
	flag.Parse()

	// 创建配置管理器
	configManager := config.NewProjectConfigManager(*configDir)

	// 列出项目
	if *listProjects {
		projects, err := configManager.ListProjects()
		if err != nil {
			log.Fatalf("❌ 获取项目列表失败: %v", err)
		}

		fmt.Println("📋 可用的项目配置:")
		fmt.Println()
		for i, project := range projects {
			fmt.Printf("  %d. %s\n", i+1, project)
			
			// 尝试获取项目信息
			if info, err := configManager.GetProjectInfo(project); err == nil {
				if desc, ok := info["description"].(string); ok && desc != "" {
					fmt.Printf("     描述: %s\n", desc)
				}
			}
		}
		fmt.Println()
		fmt.Printf("共 %d 个项目\n", len(projects))
		fmt.Println()
		fmt.Println("使用方法: sync -project <项目名>")
		return
	}

	// 检查项目名称
	if *projectName == "" {
		fmt.Println("❌ 错误: 必须指定项目名称")
		fmt.Println()
		fmt.Println("使用方法:")
		fmt.Println("  sync -project <项目名>              # 同步指定项目到 Apifox")
		fmt.Println("  sync -list                          # 列出所有可用的项目")
		fmt.Println("  sync -project <项目名> -info        # 查看项目信息")
		fmt.Println("  sync -project <项目名> -save        # 保存 OpenAPI 规范到文件")
		fmt.Println()
		os.Exit(1)
	}

	// 加载项目配置
	fmt.Printf("📖 加载项目配置: %s\n", *projectName)
	projectConfig, err := configManager.LoadProjectConfig(*projectName)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}

	// 显示项目信息
	if *showInfo {
		fmt.Println()
		fmt.Printf("=== %s 项目信息 ===\n", projectConfig.ProjectName)
		fmt.Println()
		fmt.Printf("项目名称: %s\n", projectConfig.ProjectName)
		fmt.Printf("项目描述: %s\n", projectConfig.Description)
		fmt.Printf("仓库地址: %s\n", projectConfig.RepoURL)
		fmt.Printf("本地路径: %s\n", projectConfig.LocalPath)
		fmt.Println()
		fmt.Printf("语言框架: %s\n", projectConfig.Parser.Language)
		fmt.Printf("跳过前缀: %v\n", projectConfig.Parser.SkipPrefix)
		fmt.Println()
		fmt.Printf("Apifox 项目ID: %s\n", projectConfig.Apifox.ProjectID)
		fmt.Printf("Apifox API: %s\n", projectConfig.Apifox.BaseURL)
		fmt.Printf("同步模式: %s\n", projectConfig.Apifox.SyncMode)
		fmt.Println()
		return
	}

	fmt.Printf("✓ 配置加载成功\n")
	fmt.Println()

	// 步骤 1: 解析项目
	fmt.Printf("=== 步骤 1: 解析项目 ===\n")
	fmt.Printf("项目路径: %s\n", projectConfig.LocalPath)
	fmt.Printf("解析语言: %s\n", projectConfig.Parser.Language)
	fmt.Println()

	// 检查项目路径
	if _, err := os.Stat(projectConfig.LocalPath); os.IsNotExist(err) {
		log.Fatalf("❌ 项目路径不存在: %s", projectConfig.LocalPath)
	}

	// 创建解析器
	var parser interface {
		Analyze(string) (*openapi.Spec, error)
	}

	switch projectConfig.Parser.Language {
	case "go-gin":
		parser = gin.NewGinParser()
	default:
		log.Fatalf("❌ 不支持的语言: %s", projectConfig.Parser.Language)
	}

	// 解析项目
	fmt.Println("正在解析代码...")
	spec, err := parser.Analyze(projectConfig.LocalPath)
	if err != nil {
		log.Fatalf("❌ 解析失败: %v", err)
	}

	fmt.Printf("✓ 解析完成\n")
	fmt.Printf("  - 发现 %d 个 API 端点\n", countEndpoints(spec))
	fmt.Printf("  - 发现 %d 个数据结构\n", len(spec.Components.Schemas))
	fmt.Println()

	// 步骤 2: 保存到文件（可选）
	if *saveOutput {
		fmt.Printf("=== 步骤 2: 保存 OpenAPI 规范 ===\n")
		outputDir := fmt.Sprintf(".temp/%s-output", *projectName)
		os.MkdirAll(outputDir, 0755)
		
		outputFile := fmt.Sprintf("%s/openapi.json", outputDir)
		jsonData, err := json.MarshalIndent(spec, "", "  ")
		if err != nil {
			log.Fatalf("❌ JSON 序列化失败: %v", err)
		}

		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			log.Fatalf("❌ 保存文件失败: %v", err)
		}

		fmt.Printf("✓ OpenAPI 规范已保存\n")
		fmt.Printf("  文件路径: %s\n", outputFile)
		fmt.Printf("  文件大小: %d bytes\n", len(jsonData))
		fmt.Println()
	}

	// 步骤 3: 同步到 Apifox
	fmt.Printf("=== 步骤 %d: 同步到 Apifox ===\n", func() int {
		if *saveOutput {
			return 3
		}
		return 2
	}())
	fmt.Printf("Apifox 项目ID: %s\n", projectConfig.Apifox.ProjectID)
	fmt.Printf("同步模式: %s\n", projectConfig.Apifox.SyncMode)
	fmt.Println()

	// 创建服务器配置（用于文档 URL 生成）
	serverCfg := &config.ServerConfig{
		PublicURL: "http://localhost:8080",
	}

	// 创建 Apifox 同步器
	apifoxSync := sync.NewApifoxSyncer(&projectConfig.Apifox, serverCfg)

	// 执行同步
	commitMsg := fmt.Sprintf("%s 项目文档同步", projectConfig.ProjectName)
	fmt.Println("正在同步到 Apifox...")
	
	if err := apifoxSync.Sync(spec, commitMsg); err != nil {
		log.Fatalf("❌ 同步失败: %v", err)
	}

	fmt.Println()
	fmt.Println("✓ 同步成功!")
	fmt.Println()
	fmt.Printf("=== 同步摘要 ===\n")
	fmt.Printf("项目名称: %s\n", projectConfig.ProjectName)
	fmt.Printf("API 端点: %d 个\n", countEndpoints(spec))
	fmt.Printf("数据结构: %d 个\n", len(spec.Components.Schemas))
	fmt.Println()
	fmt.Printf("📱 在 Apifox 中查看:\n")
	fmt.Printf("   https://app.apifox.com/project/%s\n", projectConfig.Apifox.ProjectID)
	fmt.Println()
}

// countEndpoints 统计端点数量
func countEndpoints(spec *openapi.Spec) int {
	count := 0
	for _, pathItem := range spec.Paths {
		if pathItem.Get != nil {
			count++
		}
		if pathItem.Post != nil {
			count++
		}
		if pathItem.Put != nil {
			count++
		}
		if pathItem.Delete != nil {
			count++
		}
		if pathItem.Patch != nil {
			count++
		}
		if pathItem.Head != nil {
			count++
		}
		if pathItem.Options != nil {
			count++
		}
	}
	return count
}

