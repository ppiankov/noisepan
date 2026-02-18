package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestHelperProcess is used to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command provided\n")
		os.Exit(2)
	}

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "entropia":
		if len(subArgs) < 2 || subArgs[0] != "scan" || subArgs[len(subArgs)-1] != "--json" {
			fmt.Fprintf(os.Stderr, "Usage: entropia scan <url> --json\n")
			os.Exit(2)
		}
		targetURL := subArgs[1]

		if targetURL == "https://timeout.com" {
			time.Sleep(2 * time.Second) // Simulate timeout (test uses short timeout)
			return
		}

		if targetURL == "https://fail.com" {
			fmt.Fprint(os.Stderr, "Scan failed due to error")
			os.Exit(1)
		}

		// Success case
		fmt.Printf(`{"url":%q,"score":{"index":75,"confidence":"high","conflict":false,"signals":["verified"]}}`, targetURL)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n", cmd)
		os.Exit(2)
	}
}

func TestGetSkipReason(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://example.com", ""},
		{"https://reddit.com/r/foo", "reddit.com not scannable"},
		{"https://www.reddit.com/r/foo", "reddit.com not scannable"},
		{"https://t.me/foo", "t.me requires auth"},
		{"invalid-url", "invalid URL"},
	}

	for _, tt := range tests {
		got := getSkipReason(tt.url)
		if got != tt.expected {
			t.Errorf("getSkipReason(%q) = %q; want %q", tt.url, got, tt.expected)
		}
	}
}

func TestRunEntropiaScan(t *testing.T) {
	// Mock execCommandContext
	oldExecCommandContext := execCommandContext
	execCommandContext = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
	defer func() { execCommandContext = oldExecCommandContext }()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		res, err := runEntropiaScan(ctx, "https://example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Score.Index != 75 {
			t.Errorf("Score.Index = %d; want 75", res.Score.Index)
		}
		if res.Score.Confidence != "high" {
			t.Errorf("Score.Confidence = %q; want high", res.Score.Confidence)
		}
	})

	t.Run("Failure", func(t *testing.T) {
		_, err := runEntropiaScan(ctx, "https://fail.com")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed: Scan failed") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})
}
