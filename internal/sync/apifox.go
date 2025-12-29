package sync

import (
	"api-doc-generator/internal/config"
	"api-doc-generator/internal/openapi"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ApifoxSyncer struct {
	cfg *config.ApifoxConfig
}

type ApifoxImportRequest struct {
	Data    *openapi.Spec          `json:"data"`
	Options ApifoxImportOptions    `json:"options"`
	Comment string                 `json:"comment,omitempty"`
}

type ApifoxImportOptions struct {
	Mode          string `json:"mode"` // merge, replace
	SyncAPIFolder bool   `json:"syncApiFolder"`
	SyncSchemas   bool   `json:"syncSchemas"`
}

func NewApifoxSyncer(cfg *config.ApifoxConfig) *ApifoxSyncer {
	return &ApifoxSyncer{cfg: cfg}
}

func (s *ApifoxSyncer) Sync(spec *openapi.Spec, commitMsg string) error {
	url := fmt.Sprintf("%s/api/v1/projects/%s/import-openapi",
		s.cfg.BaseURL, s.cfg.ProjectID)

	payload := ApifoxImportRequest{
		Data: spec,
		Options: ApifoxImportOptions{
			Mode:          "merge",
			SyncAPIFolder: true,
			SyncSchemas:   true,
		},
		Comment: fmt.Sprintf("Auto-sync: %s", commitMsg),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("apifox API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
