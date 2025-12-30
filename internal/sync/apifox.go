package sync

import (
	"api-doc-generator/internal/config"
	"api-doc-generator/internal/openapi"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type ApifoxSyncer struct {
	cfg       *config.ApifoxConfig
	serverCfg *config.ServerConfig
}

// ApifoxImportRequest 使用URL方式导入的请求结构
type ApifoxImportRequest struct {
	Input   interface{}         `json:"input"`
	Options ApifoxImportOptions `json:"options"`
}

type ApifoxImportOptions struct {
	EndpointOverwriteBehavior string `json:"endpointOverwriteBehavior"` // OVERWRITE_EXISTING, KEEP_EXISTING
	SchemaOverwriteBehavior   string `json:"schemaOverwriteBehavior"`   // OVERWRITE_EXISTING, KEEP_EXISTING
}

func NewApifoxSyncer(cfg *config.ApifoxConfig, serverCfg *config.ServerConfig) *ApifoxSyncer {
	return &ApifoxSyncer{
		cfg:       cfg,
		serverCfg: serverCfg,
	}
}

// Sync 同步OpenAPI规范到Apifox
// 1. 先保存文档到docs目录
// 2. 根据配置决定使用string还是url方式发送给Apifox
func (s *ApifoxSyncer) Sync(spec *openapi.Spec, commitMsg string) error {
	// 1. 将OpenAPI规范转换为JSON字符串
	specJSON, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}

	// 2. 先保存文档到docs目录（无论哪种方式都要保存）
	docPath, docURL, err := s.saveOpenAPIDocToPublic(string(specJSON), commitMsg)
	if err != nil {
		return fmt.Errorf("failed to save doc: %w", err)
	}

	fmt.Printf("[Apifox Sync] Document saved to: %s\n", docPath)
	fmt.Printf("[Apifox Sync] Public URL: %s\n", docURL)

	// 3. 根据配置决定同步方式
	apiURL := fmt.Sprintf("%s/v1/projects/%s/import-openapi?locale=zh-CN",
		s.cfg.BaseURL, s.cfg.ProjectID)

	var payload ApifoxImportRequest

	if s.cfg.SyncMode == "url" {
		// URL方式：发送文档的URL给Apifox
		fmt.Printf("[Apifox Sync] Using URL mode, sending URL to Apifox\n")
		payload = ApifoxImportRequest{
			Input: map[string]interface{}{
				"url": docURL,
			},
			Options: ApifoxImportOptions{
				EndpointOverwriteBehavior: "OVERWRITE_EXISTING",
				SchemaOverwriteBehavior:   "OVERWRITE_EXISTING",
			},
		}
	} else {
		// String方式：直接发送JSON内容给Apifox
		fmt.Printf("[Apifox Sync] Using string mode, sending JSON content to Apifox\n")
		payload = ApifoxImportRequest{
			Input: string(specJSON),
			Options: ApifoxImportOptions{
				EndpointOverwriteBehavior: "OVERWRITE_EXISTING",
				SchemaOverwriteBehavior:   "OVERWRITE_EXISTING",
			},
		}
	}

	return s.sendImportRequest(apiURL, payload, commitMsg)
}

// SyncByURL 从外部URL同步OpenAPI规范
// 1. 从URL下载文档内容
// 2. 保存到docs目录
// 3. 发送给Apifox（根据配置决定用string还是url方式）
func (s *ApifoxSyncer) SyncByURL(specURL string, commitMsg string) error {
	fmt.Printf("[Apifox Sync] Downloading OpenAPI spec from: %s\n", specURL)

	// 1. 从URL下载文档内容
	specJSON, err := s.downloadOpenAPIFromURL(specURL)
	if err != nil {
		return fmt.Errorf("failed to download from URL: %w", err)
	}

	// 2. 保存文档到docs目录
	docPath, docURL, err := s.saveOpenAPIDocToPublic(specJSON, commitMsg)
	if err != nil {
		return fmt.Errorf("failed to save doc: %w", err)
	}

	fmt.Printf("[Apifox Sync] Document downloaded and saved to: %s\n", docPath)
	fmt.Printf("[Apifox Sync] Public URL: %s\n", docURL)

	// 3. 根据配置决定同步方式
	apiURL := fmt.Sprintf("%s/v1/projects/%s/import-openapi?locale=zh-CN",
		s.cfg.BaseURL, s.cfg.ProjectID)

	var payload ApifoxImportRequest

	if s.cfg.SyncMode == "url" {
		// URL方式：发送我们保存的文档URL
		fmt.Printf("[Apifox Sync] Using URL mode, sending our URL to Apifox\n")
		payload = ApifoxImportRequest{
			Input: map[string]interface{}{
				"url": docURL,
			},
			Options: ApifoxImportOptions{
				EndpointOverwriteBehavior: "OVERWRITE_EXISTING",
				SchemaOverwriteBehavior:   "OVERWRITE_EXISTING",
			},
		}
	} else {
		// String方式：直接发送下载的JSON内容
		fmt.Printf("[Apifox Sync] Using string mode, sending JSON content to Apifox\n")
		payload = ApifoxImportRequest{
			Input: specJSON,
			Options: ApifoxImportOptions{
				EndpointOverwriteBehavior: "OVERWRITE_EXISTING",
				SchemaOverwriteBehavior:   "OVERWRITE_EXISTING",
			},
		}
	}

	return s.sendImportRequest(apiURL, payload, commitMsg)
}

// sendImportRequest 发送导入请求到Apifox
func (s *ApifoxSyncer) sendImportRequest(url string, payload ApifoxImportRequest, commitMsg string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 保存请求日志
	s.saveRequestLog(body, commitMsg)

	fmt.Printf("[Apifox Sync] Sending import request to Apifox...\n")

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Apifox-Api-Version", "2024-03-28")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("apifox API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	fmt.Printf("[Apifox Sync] ✅ Sync successful! Response: %s\n", string(respBody))

	// 保存响应日志
	s.saveResponseLog(respBody, commitMsg)

	return nil
}

// saveRequestLog 保存请求日志到docs目录，按项目ID和时间命名
func (s *ApifoxSyncer) saveRequestLog(body []byte, commitMsg string) {
	// 按项目ID创建目录: docs/apifox/{projectID}/
	logDir := filepath.Join("docs", "apifox", s.cfg.ProjectID)
	os.MkdirAll(logDir, 0755)

	// 文件名格式: {projectID}_{timestamp}_request.json
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(logDir, fmt.Sprintf("%s_%s_request.json", s.cfg.ProjectID, timestamp))

	logData := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"commitMsg": commitMsg,
		"request":   json.RawMessage(body),
		"projectID": s.cfg.ProjectID,
	}

	logJSON, _ := json.MarshalIndent(logData, "", "  ")
	if err := os.WriteFile(filename, logJSON, 0644); err != nil {
		fmt.Printf("[Warning] Failed to save request log: %v\n", err)
	} else {
		fmt.Printf("[Apifox Sync] Request saved to: %s\n", filename)
	}
}

// saveResponseLog 保存响应日志到docs目录，按项目ID和时间命名
func (s *ApifoxSyncer) saveResponseLog(body []byte, commitMsg string) {
	// 按项目ID创建目录: docs/apifox/{projectID}/
	logDir := filepath.Join("docs", "apifox", s.cfg.ProjectID)
	os.MkdirAll(logDir, 0755)

	// 文件名格式: {projectID}_{timestamp}_response.json
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(logDir, fmt.Sprintf("%s_%s_response.json", s.cfg.ProjectID, timestamp))

	logData := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"commitMsg": commitMsg,
		"response":  json.RawMessage(body),
		"projectID": s.cfg.ProjectID,
	}

	logJSON, _ := json.MarshalIndent(logData, "", "  ")
	if err := os.WriteFile(filename, logJSON, 0644); err != nil {
		fmt.Printf("[Warning] Failed to save response log: %v\n", err)
	} else {
		fmt.Printf("[Apifox Sync] Response saved to: %s\n", filename)
	}
}

// saveOpenAPIDocToPublic 保存OpenAPI文档到公开可访问的目录，并返回文件路径和URL
func (s *ApifoxSyncer) saveOpenAPIDocToPublic(specJSON string, commitMsg string) (string, string, error) {
	// 按项目ID创建目录: docs/apifox/{projectID}/
	docDir := filepath.Join("docs", "apifox", s.cfg.ProjectID)
	if err := os.MkdirAll(docDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 文件名格式: {projectID}_{timestamp}_openapi.json
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_openapi.json", s.cfg.ProjectID, timestamp)
	filePath := filepath.Join(docDir, filename)

	// 保存文档
	if err := os.WriteFile(filePath, []byte(specJSON), 0644); err != nil {
		return "", "", fmt.Errorf("failed to write file: %w", err)
	}

	// 生成公网可访问的URL
	// 格式: http://your-server.com/docs/apifox/{projectID}/{filename}
	docURL := fmt.Sprintf("%s/docs/apifox/%s/%s", s.serverCfg.PublicURL, s.cfg.ProjectID, filename)

	return filePath, docURL, nil
}

// downloadOpenAPIFromURL 从URL下载OpenAPI文档
func (s *ApifoxSyncer) downloadOpenAPIFromURL(url string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}
