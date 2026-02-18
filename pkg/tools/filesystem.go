package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// allowedDir restricts file operations. Set via SetAllowedDir().
var fsAllowedDir string

// SetFSAllowedDir restricts read/write/list tools to paths under the given directory.
func SetFSAllowedDir(dir string) {
	fsAllowedDir = dir
}

// checkPathAllowed validates the path is within the allowed directory.
func checkPathAllowed(rawPath string) (string, error) {
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if fsAllowedDir == "" {
		return absPath, nil
	}
	allowedAbs, err := filepath.Abs(fsAllowedDir)
	if err != nil {
		return "", fmt.Errorf("invalid allowed dir: %w", err)
	}
	// Ensure path is under the allowed dir
	if !strings.HasPrefix(absPath, allowedAbs+string(filepath.Separator)) && absPath != allowedAbs {
		return "", fmt.Errorf("access denied: path %q is outside allowed directory %q", absPath, allowedAbs)
	}
	return absPath, nil
}

type ReadFileTool struct{}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file"
}

func (t *ReadFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to read",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	safePath, err := checkPathAllowed(path)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

type WriteFileTool struct{}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file"
}

func (t *WriteFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to write",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	safePath, err := checkPathAllowed(path)
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(safePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return "File written successfully", nil
}

type ListDirTool struct{}

func (t *ListDirTool) Name() string {
	return "list_dir"
}

func (t *ListDirTool) Description() string {
	return "List files and directories in a path"
}

func (t *ListDirTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to list",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ListDirTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		path = "."
	}

	safePath, pathErr := checkPathAllowed(path)
	if pathErr != nil {
		return "", pathErr
	}
	path = safePath

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	result := ""
	for _, entry := range entries {
		if entry.IsDir() {
			result += "DIR:  " + entry.Name() + "\n"
		} else {
			result += "FILE: " + entry.Name() + "\n"
		}
	}

	return result, nil
}
