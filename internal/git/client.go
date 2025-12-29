package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Client struct {
	workDir string
}

func NewClient(workDir string) *Client {
	return &Client{workDir: workDir}
}

// CloneOrPull clones a repository if it doesn't exist, or pulls latest changes
func (c *Client) CloneOrPull(cloneURL, repoName string) (string, error) {
	// Ensure work directory exists
	if err := os.MkdirAll(c.workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}

	repoPath := filepath.Join(c.workDir, sanitizeRepoName(repoName))

	// Check if repository already exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		// Repository exists, pull latest changes
		return repoPath, c.pull(repoPath)
	}

	// Clone repository
	return repoPath, c.clone(cloneURL, repoPath)
}

func (c *Client) clone(cloneURL, repoPath string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", cloneURL, repoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (c *Client) pull(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "pull", "origin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If pull fails, try to reset and pull again
		resetCmd := exec.Command("git", "-C", repoPath, "reset", "--hard", "HEAD")
		resetCmd.Run()

		cmd = exec.Command("git", "-C", repoPath, "pull", "origin")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git pull failed: %w\nOutput: %s", err, string(output))
		}
	}
	return nil
}

// GetChangedFiles returns list of files changed in the last commit
func (c *Client) GetChangedFiles(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "diff", "--name-only", "HEAD~1", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	return files, nil
}

func sanitizeRepoName(name string) string {
	// Remove .git suffix and replace special characters
	name = strings.TrimSuffix(name, ".git")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}
