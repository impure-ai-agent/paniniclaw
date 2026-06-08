package integrations

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Tool represents an OpenAI-compatible tool definition.
type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// ToolCall represents a tool call requested by the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

// Define the schema for the terminal command tool.
var TerminalTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "execute_command",
		Description: "Execute a terminal command. The command will run in bash as a unprivileged user.",
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
func ExecuteCommand(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
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
		return output, fmt.Errorf("command execution failed: %v", err)
	}

	return output, nil
}
