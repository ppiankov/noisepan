package source

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ForgePlanSource runs forge-plan.sh and ingests suggested actions as posts.
type ForgePlanSource struct {
	scriptPath string
}

// NewForgePlan creates a forge-plan source. scriptPath must be non-empty.
func NewForgePlan(scriptPath string) (*ForgePlanSource, error) {
	if strings.TrimSpace(scriptPath) == "" {
		return nil, fmt.Errorf("forgeplan: script path is required")
	}
	return &ForgePlanSource{scriptPath: scriptPath}, nil
}

func (f *ForgePlanSource) Name() string { return "forgeplan" }

func (f *ForgePlanSource) Fetch(_ time.Time) ([]Post, error) {
	info, err := os.Stat(f.scriptPath)
	if err != nil {
		return nil, fmt.Errorf("forgeplan: script not found: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("forgeplan: %s is a directory, not a script", f.scriptPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, f.scriptPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("forgeplan: run script: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	actions := parseActions(stdout.String())
	now := time.Now()
	posts := make([]Post, 0, len(actions))
	for _, a := range actions {
		text := a.Description
		if a.Command != "" {
			text += "\n\n" + a.Command
		}
		posts = append(posts, Post{
			Source:     "forgeplan",
			Channel:    "forge-plan",
			ExternalID: fmt.Sprintf("action-%d", a.Number),
			Text:       text,
			PostedAt:   now,
		})
	}
	return posts, nil
}

type forgePlanAction struct {
	Number      int
	Description string
	Command     string
}

var actionLineRe = regexp.MustCompile(`^\s*(\d+)\.\s+(.+)$`)

func parseActions(output string) []forgePlanAction {
	lines := strings.Split(output, "\n")

	// Find "Suggested actions" section
	start := -1
	for i, line := range lines {
		if strings.Contains(strings.ToLower(strings.TrimSpace(line)), "suggested actions") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}

	var actions []forgePlanAction
	for i := start; i < len(lines); i++ {
		m := actionLineRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}

		num := 0
		_, _ = fmt.Sscanf(m[1], "%d", &num)
		desc := strings.TrimSpace(m[2])

		// Next non-empty line is the command
		cmd := ""
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed != "" {
				// Don't consume the next numbered action as a command
				if actionLineRe.MatchString(lines[j]) {
					break
				}
				cmd = trimmed
				break
			}
		}

		actions = append(actions, forgePlanAction{
			Number:      num,
			Description: desc,
			Command:     cmd,
		})
	}
	return actions
}
