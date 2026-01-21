package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProjectConfig 项目级别的配置
type ProjectConfig struct {
	ProjectName string        `json:"project_name"`
	RepoURL     string        `json:"repo_url"`
	LocalPath   string        `json:"local_path"`
	Description string        `json:"description"`
	Apifox      ApifoxConfig  `json:"apifox"`
	Parser      ParserConfig  `json:"parser"`
}

// ParserConfig 解析器配置
type ParserConfig struct {
	Language   string   `json:"language"`
	SkipPaths  []string `json:"skip_paths"`
	SkipPrefix []string `json:"skip_prefix"`
}

// ProjectConfigManager 项目配置管理器
type ProjectConfigManager struct {
	ConfigDir string
	configs   map[string]*ProjectConfig
}

// NewProjectConfigManager 创建项目配置管理器
func NewProjectConfigManager(configDir string) *ProjectConfigManager {
	if configDir == "" {
		configDir = ".temp/configs"
	}
	return &ProjectConfigManager{
		ConfigDir: configDir,
		configs:   make(map[string]*ProjectConfig),
	}
}

// LoadProjectConfig 加载指定项目的配置
func (m *ProjectConfigManager) LoadProjectConfig(projectName string) (*ProjectConfig, error) {
	// 检查缓存
	if cfg, exists := m.configs[projectName]; exists {
		return cfg, nil
	}

	// 构建配置文件路径
	configPath := filepath.Join(m.ConfigDir, projectName+".json")

	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("项目配置文件不存在: %s", configPath)
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析 JSON
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证必填字段
	if err := m.validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 缓存配置
	m.configs[projectName] = &cfg

	return &cfg, nil
}

// validateConfig 验证配置有效性
func (m *ProjectConfigManager) validateConfig(cfg *ProjectConfig) error {
	if cfg.ProjectName == "" {
		return fmt.Errorf("project_name 不能为空")
	}
	if cfg.LocalPath == "" {
		return fmt.Errorf("local_path 不能为空")
	}
	if cfg.Apifox.Token == "" {
		return fmt.Errorf("apifox.Token 不能为空")
	}
	if cfg.Apifox.ProjectID == "" {
		return fmt.Errorf("apifox.ProjectID 不能为空")
	}
	if cfg.Apifox.BaseURL == "" {
		cfg.Apifox.BaseURL = "https://api.apifox.com"
	}
	if cfg.Apifox.SyncMode == "" {
		cfg.Apifox.SyncMode = "string"
	}
	if cfg.Parser.Language == "" {
		cfg.Parser.Language = "go-gin"
	}
	return nil
}

// ListProjects 列出所有可用的项目配置
func (m *ProjectConfigManager) ListProjects() ([]string, error) {
	// 确保目录存在
	if _, err := os.Stat(m.ConfigDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("配置目录不存在: %s", m.ConfigDir)
	}

	// 读取目录
	entries, err := os.ReadDir(m.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("读取配置目录失败: %w", err)
	}

	var projects []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// 只处理 .json 文件
		if filepath.Ext(entry.Name()) == ".json" {
			// 去掉 .json 后缀
			projectName := entry.Name()[:len(entry.Name())-5]
			projects = append(projects, projectName)
		}
	}

	return projects, nil
}

// SaveProjectConfig 保存项目配置
func (m *ProjectConfigManager) SaveProjectConfig(cfg *ProjectConfig) error {
	// 验证配置
	if err := m.validateConfig(cfg); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 确保配置目录存在
	if err := os.MkdirAll(m.ConfigDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 构建配置文件路径
	configPath := filepath.Join(m.ConfigDir, cfg.ProjectName+".json")

	// 序列化为 JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	// 更新缓存
	m.configs[cfg.ProjectName] = cfg

	return nil
}

// GetProjectInfo 获取项目信息摘要
func (m *ProjectConfigManager) GetProjectInfo(projectName string) (map[string]interface{}, error) {
	cfg, err := m.LoadProjectConfig(projectName)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"project_name": cfg.ProjectName,
		"description":  cfg.Description,
		"repo_url":     cfg.RepoURL,
		"local_path":   cfg.LocalPath,
		"language":     cfg.Parser.Language,
		"apifox_id":    cfg.Apifox.ProjectID,
	}, nil
}
