package webhook

import (
	"api-doc-generator/internal/config"
	"api-doc-generator/internal/git"
	"api-doc-generator/internal/parser"
	"api-doc-generator/internal/sync"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg      *config.Config
	registry *parser.Registry
}

type GitHubWebhook struct {
	Ref        string `json:"ref"`
	Repository struct {
		Name     string `json:"name"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
	} `json:"repository"`
	Commits []struct {
		ID       string   `json:"id"`
		Message  string   `json:"message"`
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"commits"`
}

type GitLabWebhook struct {
	Ref     string `json:"ref"`
	Project struct {
		Name    string `json:"name"`
		HTTPURL string `json:"http_url"`
	} `json:"project"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"commits"`
}

type ManualTriggerRequest struct {
	RepositoryURL string `json:"repository_url" binding:"required"`
	Branch        string `json:"branch"`
	Language      string `json:"language"` // Optional: force specific parser
}

func NewHandler(cfg *config.Config, registry *parser.Registry) *Handler {
	return &Handler{
		cfg:      cfg,
		registry: registry,
	}
}

func (h *Handler) HandleGitHub(c *gin.Context) {
	log.Println("📥 Received GitHub webhook")

	// Validate webhook signature
	signature := c.GetHeader("X-Hub-Signature-256")
	if h.cfg.Webhook.Secret != "" {
		body, _ := io.ReadAll(c.Request.Body)
		if !h.validateGitHubSignature(body, signature) {
			log.Println("❌ Invalid webhook signature")
			c.JSON(401, gin.H{"error": "Invalid signature"})
			return
		}
		// Reset body for JSON binding
		c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
	}

	var webhook GitHubWebhook
	if err := c.ShouldBindJSON(&webhook); err != nil {
		log.Printf("❌ Invalid webhook payload: %v", err)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Only process main/master/develop branches
	if !strings.HasSuffix(webhook.Ref, "/main") &&
		!strings.HasSuffix(webhook.Ref, "/master") &&
		!strings.HasSuffix(webhook.Ref, "/develop") {
		log.Printf("ℹ️  Ignored: branch %s", webhook.Ref)
		c.JSON(200, gin.H{"message": "Ignored: not a tracked branch"})
		return
	}

	// Process asynchronously
	go h.processRepository(webhook.Repository.CloneURL, webhook.Repository.Name, webhook.Commits)

	c.JSON(200, gin.H{"message": "Processing started", "repository": webhook.Repository.Name})
}

func (h *Handler) HandleGitLab(c *gin.Context) {
	log.Println("📥 Received GitLab webhook")

	var webhook GitLabWebhook
	if err := c.ShouldBindJSON(&webhook); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Only process main/master/develop branches
	if !strings.HasSuffix(webhook.Ref, "/main") &&
		!strings.HasSuffix(webhook.Ref, "/master") &&
		!strings.HasSuffix(webhook.Ref, "/develop") {
		c.JSON(200, gin.H{"message": "Ignored: not a tracked branch"})
		return
	}

	go h.processRepository(webhook.Project.HTTPURL, webhook.Project.Name, nil)

	c.JSON(200, gin.H{"message": "Processing started", "repository": webhook.Project.Name})
}

func (h *Handler) ManualTrigger(c *gin.Context) {
	var req ManualTriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	log.Printf("🔧 Manual trigger for: %s", req.RepositoryURL)

	// Extract repo name from URL
	parts := strings.Split(strings.TrimSuffix(req.RepositoryURL, ".git"), "/")
	repoName := parts[len(parts)-1]

	go h.processRepository(req.RepositoryURL, repoName, nil)

	c.JSON(200, gin.H{
		"message":    "Processing started",
		"repository": repoName,
	})
}

func (h *Handler) processRepository(cloneURL, repoName string, commits interface{}) {
	log.Printf("🔄 Processing repository: %s", repoName)

	// 1. Clone/pull repository
	gitClient := git.NewClient(h.cfg.Git.WorkDir)
	repoPath, err := gitClient.CloneOrPull(cloneURL, repoName)
	if err != nil {
		log.Printf("❌ Git clone/pull failed: %v", err)
		return
	}

	// 2. Detect language and select parser
	language := detectLanguage(repoPath)
	log.Printf("🔍 Detected language: %s", language)

	p, err := h.registry.Get(language)
	if err != nil {
		log.Printf("⚠️  No parser available for: %s", language)
		return
	}

	// 3. Analyze code and generate OpenAPI
	log.Printf("🔍 Analyzing code with %s parser...", p.Name())
	spec, err := p.Analyze(repoPath)
	if err != nil {
		log.Printf("❌ Code analysis failed: %v", err)
		return
	}

	log.Printf("✅ Generated OpenAPI spec with %d paths", len(spec.Paths))

	// 4. Sync to Apifox
	if h.cfg.Apifox.Token == "" || h.cfg.Apifox.ProjectID == "" {
		log.Println("⚠️  Apifox credentials not configured, skipping sync")
		return
	}

	syncer := sync.NewApifoxSyncer(h.cfg.Apifox)
	commitMsg := extractCommitMessage(commits)

	log.Printf("📤 Syncing to Apifox...")
	err = syncer.Sync(spec, commitMsg)
	if err != nil {
		log.Printf("❌ Apifox sync failed: %v", err)
		return
	}

	log.Printf("✅ Successfully synced %s to Apifox", repoName)
}

func (h *Handler) validateGitHubSignature(body []byte, signature string) bool {
	if h.cfg.Webhook.Secret == "" {
		return true
	}

	mac := hmac.New(sha256.New, []byte(h.cfg.Webhook.Secret))
	mac.Write(body)
	expectedMAC := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

func detectLanguage(repoPath string) string {
	// Check for Go project
	if fileExists(repoPath + "/go.mod") {
		return "go-gin"
	}

	// Future: Add detection for other languages
	// if fileExists(repoPath + "/package.json") {
	//     return "node-express"
	// }
	// if fileExists(repoPath + "/requirements.txt") {
	//     return "python-fastapi"
	// }

	return "unknown"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func extractCommitMessage(commits interface{}) string {
	if commits == nil {
		return "Manual sync"
	}

	switch v := commits.(type) {
	case []struct {
		ID       string
		Message  string
		Added    []string
		Modified []string
		Removed  []string
	}:
		if len(v) > 0 {
			return v[0].Message
		}
	}

	return "Code update"
}
