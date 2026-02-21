package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ppiankov/noisepan/internal/store"
)

func TestPrintStats(t *testing.T) {
	stats := []store.ChannelStats{
		{Source: "rss", Channel: "CISA", Total: 47, ReadNow: 31, Skim: 12, Ignored: 4,
			FirstSeen: time.Now().AddDate(0, 0, -60), LastSeen: time.Now()},
		{Source: "rss", Channel: "r/devops", Total: 100, ReadNow: 5, Skim: 15, Ignored: 80,
			FirstSeen: time.Now().AddDate(0, 0, -45), LastSeen: time.Now()},
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	printStats(w, stats, 30*24*time.Hour)
	_ = w.Close()

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	_ = r.Close()

	if !strings.Contains(output, "147 posts from 2 channels") {
		t.Errorf("header missing totals, got:\n%s", output)
	}
	if !strings.Contains(output, "Signal-to-Noise") {
		t.Error("missing signal-to-noise section")
	}
	if !strings.Contains(output, "CISA") {
		t.Error("missing CISA channel")
	}
	if !strings.Contains(output, "Scoring Distribution") {
		t.Error("missing scoring distribution section")
	}
	if !strings.Contains(output, "Read Now:") {
		t.Error("missing read now count")
	}
}

func TestPrintStats_StaleChannels(t *testing.T) {
	staleTime := time.Now().AddDate(0, 0, -14)
	stats := []store.ChannelStats{
		{Source: "rss", Channel: "active-feed", Total: 10, ReadNow: 3, Skim: 5, Ignored: 2,
			FirstSeen: time.Now().AddDate(0, 0, -60), LastSeen: time.Now()},
		{Source: "rss", Channel: "stale-feed", Total: 5, ReadNow: 0, Skim: 1, Ignored: 4,
			FirstSeen: time.Now().AddDate(0, 0, -60), LastSeen: staleTime},
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	printStats(w, stats, 30*24*time.Hour)
	_ = w.Close()

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	_ = r.Close()

	if !strings.Contains(output, "Stale Channels") {
		t.Error("missing stale channels section")
	}
	if !strings.Contains(output, "stale-feed") {
		t.Error("missing stale-feed in stale section")
	}
	// active-feed should not have "days ago" after its name in stale section
	if strings.Contains(output, "active-feed — last post") {
		t.Error("active-feed should not appear in stale section")
	}
}

func TestPrintStats_MaturityAnnotation(t *testing.T) {
	stats := []store.ChannelStats{
		{Source: "rss", Channel: "young-feed", Total: 10, ReadNow: 0, Skim: 0, Ignored: 10,
			FirstSeen: time.Now().AddDate(0, 0, -5), LastSeen: time.Now()},
		{Source: "rss", Channel: "mature-feed", Total: 50, ReadNow: 20, Skim: 10, Ignored: 20,
			FirstSeen: time.Now().AddDate(0, 0, -60), LastSeen: time.Now()},
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	printStats(w, stats, 30*24*time.Hour)
	_ = w.Close()

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	_ = r.Close()

	// young-feed should have maturity annotation
	if !strings.Contains(output, "5d data") {
		t.Errorf("expected maturity annotation for young feed, got:\n%s", output)
	}
	// mature-feed should NOT have maturity annotation
	if strings.Contains(output, "60d data") {
		t.Errorf("mature feed should not have maturity annotation, got:\n%s", output)
	}
}

func TestPrintStatsJSON(t *testing.T) {
	stats := []store.ChannelStats{
		{Source: "rss", Channel: "CISA", Total: 47, ReadNow: 31, Skim: 12, Ignored: 4,
			FirstSeen: time.Now().AddDate(0, 0, -45), LastSeen: time.Now()},
		{Source: "rss", Channel: "r/devops", Total: 100, ReadNow: 5, Skim: 15, Ignored: 80,
			FirstSeen: time.Now().AddDate(0, 0, -10), LastSeen: time.Now()},
	}

	var buf bytes.Buffer
	if err := printStatsJSON(&buf, stats, 30*24*time.Hour); err != nil {
		t.Fatalf("print stats json: %v", err)
	}

	var got jsonStatsOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("parse json: %v\noutput:\n%s", err, buf.String())
	}

	if len(got.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(got.Channels))
	}

	// Check first channel
	cisa := got.Channels[0]
	if cisa.Channel != "CISA" || cisa.Source != "rss" {
		t.Errorf("unexpected channel: %+v", cisa)
	}
	if cisa.Total != 47 || cisa.ReadNow != 31 || cisa.Skim != 12 || cisa.Ignored != 4 {
		t.Errorf("unexpected counts: %+v", cisa)
	}
	if cisa.DataDays < 44 || cisa.DataDays > 46 {
		t.Errorf("data_days = %d, want ~45", cisa.DataDays)
	}

	// Check distribution
	if got.Distribution.Total != 147 {
		t.Errorf("total = %d, want 147", got.Distribution.Total)
	}
	if got.Distribution.ReadNow != 36 {
		t.Errorf("read_now = %d, want 36", got.Distribution.ReadNow)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		err   bool
	}{
		{"30d", 30 * 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"48h", 48 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"bad", 0, true},
	}

	for _, tt := range tests {
		got, err := parseDuration(tt.input)
		if tt.err && err == nil {
			t.Errorf("parseDuration(%q) expected error", tt.input)
			continue
		}
		if !tt.err && err != nil {
			t.Errorf("parseDuration(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSignalPct(t *testing.T) {
	tests := []struct {
		cs   store.ChannelStats
		want float64
	}{
		{store.ChannelStats{Total: 100, ReadNow: 20, Skim: 30}, 50},
		{store.ChannelStats{Total: 0}, 0},
		{store.ChannelStats{Total: 10, ReadNow: 10}, 100},
		{store.ChannelStats{Total: 10, Ignored: 10}, 0},
	}

	for _, tt := range tests {
		got := signalPct(tt.cs)
		if got != tt.want {
			t.Errorf("signalPct(%+v) = %v, want %v", tt.cs, got, tt.want)
		}
	}
}
