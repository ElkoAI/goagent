package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Default timeout for exec commands.
const execTimeout = 30 * time.Second

// Exec runs a shell command and returns stdout/stderr.
// This is the bridge that lets OpenClaw-style skills work —
// skills instruct the agent to call exec with shell commands.
func Exec(params json.RawMessage) (any, error) {
	var req struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"` // seconds, optional
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}

	if req.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Block obviously dangerous patterns.
	if err := validateCommand(req.Command); err != nil {
		return nil, err
	}

	timeout := execTimeout
	if req.Timeout > 0 && req.Timeout <= 300 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", req.Command)
	output, err := cmd.CombinedOutput()

	result := map[string]any{
		"command":   req.Command,
		"output":    strings.TrimSpace(string(output)),
		"exit_code": cmd.ProcessState.ExitCode(),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result["error"] = "command timed out"
		} else {
			result["error"] = err.Error()
		}
	}

	return result, nil
}

// validateCommand rejects commands that are destructive or dangerous.
func validateCommand(cmd string) error {
	blocked := []string{
		"rm -rf /",
		"mkfs",
		"dd if=",
		":(){",       // fork bomb
		"> /dev/sd",  // overwrite disk
		"chmod -R 777 /",
	}
	lower := strings.ToLower(cmd)
	for _, pattern := range blocked {
		if strings.Contains(lower, pattern) {
			return fmt.Errorf("command blocked: contains dangerous pattern %q", pattern)
		}
	}
	return nil
}
