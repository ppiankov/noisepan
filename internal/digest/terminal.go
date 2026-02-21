package digest

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ppiankov/noisepan/internal/taste"
)

// TerminalFormatter formats a digest for terminal output.
type TerminalFormatter struct {
	color bool
}

// NewTerminal creates a terminal formatter. Set color=true for ANSI colors.
func NewTerminal(color bool) *TerminalFormatter {
	return &TerminalFormatter{color: color}
}

// Format writes the digest to w grouped by tier.
func (f *TerminalFormatter) Format(w io.Writer, input DigestInput) error {
	readNow, skims, ignoreCount := groupByTier(input.Items)

	// Header
	sinceStr := formatDuration(input.Since)
	header := fmt.Sprintf("noisepan — %d channels, %d posts, since %s",
		input.Channels, input.TotalPosts, sinceStr)
	fmt.Fprintln(w, f.bold(header))
	fmt.Fprintln(w)

	if len(readNow) == 0 && len(skims) == 0 && ignoreCount == 0 {
		fmt.Fprintln(w, "No posts found.")
		return nil
	}

	// Trending section
	if len(input.Trending) > 0 {
		fmt.Fprintln(w, f.bold(fmt.Sprintf("--- Trending (appeared in %d+ sources) ---", 3)))
		fmt.Fprintln(w)
		for _, tr := range input.Trending {
			fmt.Fprintf(w, "  %s — mentioned in %d channels\n",
				f.bold(fmt.Sprintf("%q", tr.Keyword)), len(tr.Channels))
			fmt.Fprintf(w, "    %s\n", f.dim(strings.Join(tr.Channels, ", ")))
		}
		fmt.Fprintln(w)
	}

	// Read Now section
	if len(readNow) > 0 {
		fmt.Fprintln(w, f.green(f.bold(fmt.Sprintf("--- Read Now (%d) ---", len(readNow)))))
		fmt.Fprintln(w)
		for _, item := range readNow {
			f.writeReadNowItem(w, item)
		}
	}

	// Skim section
	if len(skims) > 0 {
		fmt.Fprintln(w, f.yellow(f.bold(fmt.Sprintf("--- Skim (%d) ---", len(skims)))))
		fmt.Fprintln(w)
		for _, item := range skims {
			f.writeSkimItem(w, item)
		}
		fmt.Fprintln(w)
	}

	// Footer
	if ignoreCount > 0 {
		fmt.Fprintln(w, f.dim(fmt.Sprintf("Ignored: %d posts (noise suppressed)", ignoreCount)))
	}

	return nil
}

func (f *TerminalFormatter) writeReadNowItem(w io.Writer, item DigestItem) {
	labels := ""
	if len(item.Labels) > 0 {
		labels = " [" + strings.Join(item.Labels, ", ") + "]"
	}

	firstBullet := ""
	if len(item.Summary.Bullets) > 0 {
		firstBullet = item.Summary.Bullets[0]
	}

	fmt.Fprintf(w, "  %s%s %s — %s\n",
		f.bold(fmt.Sprintf("[%d]", item.Score)),
		f.dim(labels),
		item.Post.Channel,
		firstBullet,
	)

	// Additional bullets indented
	for _, bullet := range item.Summary.Bullets[1:] {
		fmt.Fprintf(w, "      %s\n", f.dim(bullet))
	}
	if item.Post.URL != "" {
		fmt.Fprintf(w, "      %s\n", f.dim(item.Post.URL))
	}
	if len(item.AlsoIn) > 0 {
		fmt.Fprintf(w, "      %s\n", f.dim("also in: "+strings.Join(item.AlsoIn, ", ")))
	}
	fmt.Fprintln(w)
}

func (f *TerminalFormatter) writeSkimItem(w io.Writer, item DigestItem) {
	firstBullet := ""
	if len(item.Summary.Bullets) > 0 {
		firstBullet = item.Summary.Bullets[0]
	}

	fmt.Fprintf(w, "  [%d] %s — %s\n", item.Score, item.Post.Channel, firstBullet)
	if item.Post.URL != "" {
		fmt.Fprintf(w, "      %s\n", f.dim(item.Post.URL))
	}
	if len(item.AlsoIn) > 0 {
		fmt.Fprintf(w, "      %s\n", f.dim("also in: "+strings.Join(item.AlsoIn, ", ")))
	}
}

func groupByTier(items []DigestItem) (readNow, skims []DigestItem, ignoreCount int) {
	for _, item := range items {
		switch item.Tier {
		case taste.TierReadNow:
			readNow = append(readNow, item)
		case taste.TierSkim:
			skims = append(skims, item)
		default:
			ignoreCount++
		}
	}
	return
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 24 && hours%24 == 0 {
		return fmt.Sprintf("%dd", hours/24)
	}
	return fmt.Sprintf("%dh", hours)
}

// ANSI helpers — no-op when color=false.

func (f *TerminalFormatter) bold(s string) string {
	if !f.color {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

func (f *TerminalFormatter) green(s string) string {
	if !f.color {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func (f *TerminalFormatter) yellow(s string) string {
	if !f.color {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

func (f *TerminalFormatter) dim(s string) string {
	if !f.color {
		return s
	}
	return "\033[2m" + s + "\033[0m"
}
