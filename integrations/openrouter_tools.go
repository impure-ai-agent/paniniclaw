package integrations

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Tool represents an OpenAI-compatible tool definition.
type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function,omitzero"`
}

type FunctionDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// Define the schema for the terminal command tool.
var TerminalTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "execute_command",
		Description: "Execute a terminal command. The command will run in bash as a unprivileged user. Commands are automatically killed after 30 seconds if they hang.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The bash command to execute.",
				},
			},
			"required": []string{"command"},
		},
	},
}

type ExecuteCommandArgs struct {
	Command string `json:"command"`
}

// ExecuteCommand runs a bash command and returns the combined output (stdout and stderr).
// Commands are killed after 30 seconds to prevent hanging.
func ExecuteCommand(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	errStr := stderr.String()

	if errStr != "" {
		if output != "" {
			output += "\nStderr:\n" + errStr
		} else {
			output = "Stderr:\n" + errStr
		}
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output, fmt.Errorf("command timed out after 30 seconds: %v", err)
		}
		return output, fmt.Errorf("command execution failed: %v", err)
	}

	return output, nil
}

// Define the schema for the append_notes tool.
var AppendNotesTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "append_notes",
		Description: "Append a new line of text to the user's notes file (notes/user<id>.md) or to the general notes file (notes/general.md). Use this whenever the user has to explain things or corrects you.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The text to append to the notes file.",
				},
				"scope": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"user", "general"},
					"description": "Whether to append to the user's personal notes (user) or the general notes (general).",
				},
			},
			"required": []string{"content", "scope"},
		},
	},
}

type AppendNotesArgs struct {
	Content string `json:"content"`
	Scope   string `json:"scope"`
}

// AppendNotes appends a new line to the specified notes file.
func AppendNotes(content string, scope string, userId int) (string, error) {
	var path string
	if scope == "general" {
		path = "notes/general.md"
	} else {
		path = fmt.Sprintf("notes/user%d.md", userId)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create notes directory: %v", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open notes file: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content + "\n"); err != nil {
		return "", fmt.Errorf("failed to write to notes file: %v", err)
	}

	return fmt.Sprintf("Appended to %s", path), nil
}

// Define the schema for the end_task tool.
var EndTaskTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "end_task",
		Description: "Mark the currently active scheduled task as complete. If you encountered an issue and could not complete the objective, provide a 'reason' parameter explaining why.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "If set, marks the task as failed with this explanation. Omit or leave empty for a successful completion.",
				},
			},
		},
	},
}

type EndTaskArgs struct {
	Reason string `json:"reason"`
}
