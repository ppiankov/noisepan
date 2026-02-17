package privacy

import (
	"testing"
)

func TestCompile_Valid(t *testing.T) {
	patterns, err := Compile([]string{`(?i)token`, `\bsecret\b`})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(patterns) != 2 {
		t.Errorf("got %d patterns, want 2", len(patterns))
	}
}

func TestCompile_Invalid(t *testing.T) {
	_, err := Compile([]string{`[invalid`})
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestCompile_Empty(t *testing.T) {
	patterns, err := Compile(nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(patterns) != 0 {
		t.Errorf("got %d patterns, want 0", len(patterns))
	}
}

func TestApply_SinglePattern(t *testing.T) {
	patterns, _ := Compile([]string{`(?i)token`})
	result := Apply("My API Token is abc123", patterns)
	want := "My API [REDACTED] is abc123"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestApply_MultiplePatterns(t *testing.T) {
	patterns, _ := Compile([]string{`(?i)token`, `(?i)secret`})
	result := Apply("Token and Secret values", patterns)
	want := "[REDACTED] and [REDACTED] values"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestApply_MultipleMatches(t *testing.T) {
	patterns, _ := Compile([]string{`(?i)password`})
	result := Apply("password is password", patterns)
	want := "[REDACTED] is [REDACTED]"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestApply_NoMatch(t *testing.T) {
	patterns, _ := Compile([]string{`(?i)token`})
	text := "nothing to redact here"
	result := Apply(text, patterns)
	if result != text {
		t.Errorf("got %q, want unchanged", result)
	}
}

func TestApply_EmptyPatterns(t *testing.T) {
	text := "should not change"
	result := Apply(text, nil)
	if result != text {
		t.Errorf("got %q, want unchanged", result)
	}
}
