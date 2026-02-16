package digest

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ppiankov/noisepan/internal/source"
	"github.com/ppiankov/noisepan/internal/summarize"
	"github.com/ppiankov/noisepan/internal/taste"
)

func makeItem(tier string, score int, channel string, labels []string, bullets []string) DigestItem {
	return DigestItem{
		ScoredPost: taste.ScoredPost{
			Post:   source.Post{Source: "telegram", Channel: channel, ExternalID: "1"},
			Score:  score,
			Labels: labels,
			Tier:   tier,
		},
		Summary: summarize.Summary{Bullets: bullets},
	}
}

func TestFormat_FullDigest(t *testing.T) {
	f := NewTerminal(false)
	var buf bytes.Buffer

	input := DigestInput{
		Items: []DigestItem{
			makeItem(taste.TierReadNow, 10, "security", []string{"critical"}, []string{"CVE-2026-1234 found", "Patch available"}),
			makeItem(taste.TierSkim, 4, "devops", nil, []string{"New Helm chart"}),
			makeItem(taste.TierIgnore, -2, "spam", nil, []string{"Buy now"}),
		},
		Channels:   3,
		TotalPosts: 50,
		Since:      24 * time.Hour,
	}

	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	out := buf.String()

	// Header
	if !strings.Contains(out, "noisepan") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "3 channels") {
		t.Error("missing channel count")
	}
	if !strings.Contains(out, "50 posts") {
		t.Error("missing post count")
	}

	// Read Now section
	if !strings.Contains(out, "Read Now (1)") {
		t.Error("missing Read Now section")
	}
	if !strings.Contains(out, "[10]") {
		t.Error("missing score")
	}
	if !strings.Contains(out, "critical") {
		t.Error("missing labels")
	}
	if !strings.Contains(out, "CVE-2026-1234") {
		t.Error("missing first bullet")
	}
	if !strings.Contains(out, "Patch available") {
		t.Error("missing second bullet")
	}

	// Skim section
	if !strings.Contains(out, "Skim (1)") {
		t.Error("missing Skim section")
	}
	if !strings.Contains(out, "New Helm chart") {
		t.Error("missing skim bullet")
	}

	// Footer
	if !strings.Contains(out, "Ignored: 1 posts") {
		t.Error("missing ignore footer")
	}
}

func TestFormat_ReadNowDetails(t *testing.T) {
	f := NewTerminal(false)
	var buf bytes.Buffer

	input := DigestInput{
		Items: []DigestItem{
			makeItem(taste.TierReadNow, 8, "ops", []string{"certs", "ops"}, []string{"Cert expired", "Rotation needed", "See docs"}),
		},
		Channels:   1,
		TotalPosts: 1,
		Since:      12 * time.Hour,
	}

	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[8]") {
		t.Error("missing score [8]")
	}
	if !strings.Contains(out, "certs, ops") {
		t.Error("missing labels")
	}
	if !strings.Contains(out, "Rotation needed") {
		t.Error("missing second bullet")
	}
	if !strings.Contains(out, "See docs") {
		t.Error("missing third bullet")
	}
}

func TestFormat_SkimOneLiner(t *testing.T) {
	f := NewTerminal(false)
	var buf bytes.Buffer

	input := DigestInput{
		Items: []DigestItem{
			makeItem(taste.TierSkim, 4, "k8s", nil, []string{"New release"}),
		},
		Channels:   1,
		TotalPosts: 1,
		Since:      24 * time.Hour,
	}

	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[4] k8s") {
		t.Errorf("output = %q, want containing '[4] k8s'", out)
	}
}

func TestFormat_IgnoreCount(t *testing.T) {
	f := NewTerminal(false)
	var buf bytes.Buffer

	input := DigestInput{
		Items: []DigestItem{
			makeItem(taste.TierIgnore, -1, "spam1", nil, []string{"ad"}),
			makeItem(taste.TierIgnore, -3, "spam2", nil, []string{"ad"}),
			makeItem(taste.TierIgnore, 0, "spam3", nil, []string{"ad"}),
		},
		Channels:   3,
		TotalPosts: 3,
		Since:      24 * time.Hour,
	}

	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	if !strings.Contains(buf.String(), "Ignored: 3 posts") {
		t.Errorf("output = %q, want containing 'Ignored: 3 posts'", buf.String())
	}
}

func TestFormat_EmptyInput(t *testing.T) {
	f := NewTerminal(false)
	var buf bytes.Buffer

	input := DigestInput{
		Channels:   2,
		TotalPosts: 0,
		Since:      24 * time.Hour,
	}

	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "noisepan") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "No posts found") {
		t.Error("missing 'No posts found' message")
	}
}

func TestFormat_NoReadNow(t *testing.T) {
	f := NewTerminal(false)
	var buf bytes.Buffer

	input := DigestInput{
		Items: []DigestItem{
			makeItem(taste.TierSkim, 3, "ch", nil, []string{"something"}),
			makeItem(taste.TierIgnore, -1, "ch", nil, []string{"noise"}),
		},
		Channels:   1,
		TotalPosts: 2,
		Since:      48 * time.Hour,
	}

	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Read Now") {
		t.Error("should not have Read Now section")
	}
	if !strings.Contains(out, "Skim (1)") {
		t.Error("missing Skim section")
	}
	if !strings.Contains(out, "since 2d") {
		t.Error("missing time window")
	}
}

func TestFormat_NoANSIWithoutColor(t *testing.T) {
	f := NewTerminal(false)
	var buf bytes.Buffer

	input := DigestInput{
		Items: []DigestItem{
			makeItem(taste.TierReadNow, 10, "ch", []string{"ops"}, []string{"test"}),
		},
		Channels:   1,
		TotalPosts: 1,
		Since:      24 * time.Hour,
	}

	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	if strings.Contains(buf.String(), "\033[") {
		t.Error("found ANSI escape codes with color=false")
	}
}

func TestFormat_DurationDays(t *testing.T) {
	f := NewTerminal(false)
	var buf bytes.Buffer

	input := DigestInput{
		Channels:   1,
		TotalPosts: 0,
		Since:      72 * time.Hour,
	}

	if err := f.Format(&buf, input); err != nil {
		t.Fatalf("format: %v", err)
	}

	if !strings.Contains(buf.String(), "since 3d") {
		t.Errorf("output = %q, want containing 'since 3d'", buf.String())
	}
}
