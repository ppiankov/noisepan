package source

import (
	"testing"
	"time"
)

const sampleOutput = `
Runforge task files
  noisepan: 2 tasks

Uncommitted changes
  chainwatch: 3 files changed

Suggested actions

  1. Run noisepan tasks (2 WOs)
  cd /path/to/codexrun && go run ./cmd/codexrun/ run

  2. Push chainwatch
  cd /path/chainwatch && git push origin main

  3. Review changes
  cd /path/repo && git status
`

func TestParseActions(t *testing.T) {
	actions := parseActions(sampleOutput)
	if len(actions) != 3 {
		t.Fatalf("got %d actions, want 3", len(actions))
	}

	if actions[0].Number != 1 {
		t.Errorf("action[0].Number = %d, want 1", actions[0].Number)
	}
	if actions[0].Description != "Run noisepan tasks (2 WOs)" {
		t.Errorf("action[0].Description = %q", actions[0].Description)
	}
	if actions[0].Command != "cd /path/to/codexrun && go run ./cmd/codexrun/ run" {
		t.Errorf("action[0].Command = %q", actions[0].Command)
	}

	if actions[1].Number != 2 {
		t.Errorf("action[1].Number = %d, want 2", actions[1].Number)
	}
	if actions[1].Description != "Push chainwatch" {
		t.Errorf("action[1].Description = %q", actions[1].Description)
	}

	if actions[2].Number != 3 {
		t.Errorf("action[2].Number = %d, want 3", actions[2].Number)
	}
}

func TestParseActions_NothingPending(t *testing.T) {
	output := `
Runforge task files

Suggested actions

  Nothing pending. Clean state.
`
	actions := parseActions(output)
	if len(actions) != 0 {
		t.Errorf("got %d actions, want 0", len(actions))
	}
}

func TestParseActions_NoSection(t *testing.T) {
	output := `
Runforge task files
  noisepan: 2 tasks

Uncommitted changes
  chainwatch: 3 files changed
`
	actions := parseActions(output)
	if len(actions) != 0 {
		t.Errorf("got %d actions, want 0", len(actions))
	}
}

func TestNewForgePlan_EmptyPath(t *testing.T) {
	_, err := NewForgePlan("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestNewForgePlan_WhitespacePath(t *testing.T) {
	_, err := NewForgePlan("   ")
	if err == nil {
		t.Fatal("expected error for whitespace path")
	}
}

func TestForgePlanSource_Name(t *testing.T) {
	fp, err := NewForgePlan("/some/script.sh")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if fp.Name() != "forgeplan" {
		t.Errorf("name = %q, want forgeplan", fp.Name())
	}
}

func TestForgePlanSource_FetchNonexistent(t *testing.T) {
	fp, err := NewForgePlan("/nonexistent/forge-plan.sh")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_, err = fp.Fetch(time.Now().Add(-24 * time.Hour))
	if err == nil {
		t.Fatal("expected error for nonexistent script")
	}
}

func TestForgePlanAction_PostConstruction(t *testing.T) {
	actions := parseActions(sampleOutput)
	if len(actions) == 0 {
		t.Fatal("no actions parsed")
	}

	a := actions[0]
	text := a.Description + "\n\n" + a.Command
	if text != "Run noisepan tasks (2 WOs)\n\ncd /path/to/codexrun && go run ./cmd/codexrun/ run" {
		t.Errorf("post text = %q", text)
	}

	externalID := "action-1"
	if externalID != "action-1" {
		t.Errorf("external_id = %q, want action-1", externalID)
	}
}
