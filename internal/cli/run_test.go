package cli

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestParseRunEvery(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "empty",
			input:   "",
			want:    0,
			wantErr: false,
		},
		{
			name:    "valid duration",
			input:   "30m",
			want:    30 * time.Minute,
			wantErr: false,
		},
		{
			name:    "parse error",
			input:   "abc",
			want:    0,
			wantErr: true,
		},
		{
			name:    "zero duration",
			input:   "0s",
			want:    0,
			wantErr: true,
		},
		{
			name:    "negative duration",
			input:   "-1m",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		got, err := parseRunEvery(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", tt.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tt.name, err)
		}
		if got != tt.want {
			t.Fatalf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestRunActionRunsOnceWithoutEvery(t *testing.T) {
	oldEvery := runEvery
	oldPull := runPullAction
	oldDigest := runDigestAction
	t.Cleanup(func() {
		runEvery = oldEvery
		runPullAction = oldPull
		runDigestAction = oldDigest
	})

	runEvery = ""
	pullCalls := 0
	digestCalls := 0

	runPullAction = func(_ *cobra.Command, _ []string) error {
		pullCalls++
		return nil
	}
	runDigestAction = func(_ *cobra.Command, _ []string) error {
		digestCalls++
		return nil
	}

	if err := runAction(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runAction failed: %v", err)
	}

	if pullCalls != 1 {
		t.Fatalf("pull called %d times, want 1", pullCalls)
	}
	if digestCalls != 1 {
		t.Fatalf("digest called %d times, want 1", digestCalls)
	}
}

func TestRunActionWatchModeImmediateThenInterval(t *testing.T) {
	oldEvery := runEvery
	oldPull := runPullAction
	oldDigest := runDigestAction
	t.Cleanup(func() {
		runEvery = oldEvery
		runPullAction = oldPull
		runDigestAction = oldDigest
	})

	interval := 80 * time.Millisecond
	runEvery = interval.String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	pullTimes := make([]time.Time, 0, 2)
	digestCalls := 0

	runPullAction = func(_ *cobra.Command, _ []string) error {
		mu.Lock()
		pullTimes = append(pullTimes, time.Now())
		mu.Unlock()
		return nil
	}
	runDigestAction = func(_ *cobra.Command, _ []string) error {
		mu.Lock()
		digestCalls++
		count := digestCalls
		mu.Unlock()

		if count >= 2 {
			cancel()
		}
		return nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	start := time.Now()

	if err := runAction(cmd, nil); err != nil {
		t.Fatalf("runAction failed: %v", err)
	}

	mu.Lock()
	gotTimes := append([]time.Time(nil), pullTimes...)
	gotDigestCalls := digestCalls
	mu.Unlock()

	if gotDigestCalls < 2 {
		t.Fatalf("digest called %d times, want at least 2", gotDigestCalls)
	}
	if len(gotTimes) < 2 {
		t.Fatalf("pull called %d times, want at least 2", len(gotTimes))
	}

	if firstDelay := gotTimes[0].Sub(start); firstDelay >= interval {
		t.Fatalf("first run delayed by %v, want less than %v", firstDelay, interval)
	}

	minGap := interval - 10*time.Millisecond
	if secondGap := gotTimes[1].Sub(gotTimes[0]); secondGap < minGap {
		t.Fatalf("interval gap too short: got %v, want at least %v", secondGap, minGap)
	}
}

func TestRunWatchStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	calls := 0
	start := time.Now()
	err := runWatch(ctx, 10*time.Second, func() error {
		calls++
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("runWatch failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("runOnce called %d times, want 1", calls)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("watch shutdown took too long: %v", elapsed)
	}
}
