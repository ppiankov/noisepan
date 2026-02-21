package digest

import (
	"fmt"
	"io"
	"strings"
)

// MarkdownFormatter formats a digest as Markdown.
type MarkdownFormatter struct{}

// NewMarkdown creates a Markdown formatter.
func NewMarkdown() *MarkdownFormatter {
	return &MarkdownFormatter{}
}

// Format writes the digest as Markdown to w.
func (f *MarkdownFormatter) Format(w io.Writer, input DigestInput) error {
	readNow, skims, ignoreCount := groupByTier(input.Items)

	sinceStr := formatDuration(input.Since)
	fmt.Fprintf(w, "# noisepan digest\n\n")
	fmt.Fprintf(w, "%d channels, %d posts, since %s\n\n", input.Channels, input.TotalPosts, sinceStr)

	if len(readNow) == 0 && len(skims) == 0 && ignoreCount == 0 {
		fmt.Fprintln(w, "No posts found.")
		return nil
	}

	if len(input.Trending) > 0 {
		fmt.Fprintf(w, "## Trending (appeared in %d+ sources)\n\n", 3)
		for _, tr := range input.Trending {
			fmt.Fprintf(w, "- **%q** — mentioned in %d channels: %s\n",
				tr.Keyword, len(tr.Channels), strings.Join(tr.Channels, ", "))
		}
		fmt.Fprintln(w)
	}

	if len(readNow) > 0 {
		fmt.Fprintf(w, "## Read Now (%d)\n\n", len(readNow))
		for _, item := range readNow {
			f.writeReadNowItem(w, item)
		}
	}

	if len(skims) > 0 {
		fmt.Fprintf(w, "## Skim (%d)\n\n", len(skims))
		for _, item := range skims {
			f.writeSkimItem(w, item)
		}
		fmt.Fprintln(w)
	}

	if ignoreCount > 0 {
		fmt.Fprintf(w, "*Ignored: %d posts*\n", ignoreCount)
	}

	return nil
}

func (f *MarkdownFormatter) writeReadNowItem(w io.Writer, item DigestItem) {
	headline := ""
	if len(item.Summary.Bullets) > 0 {
		headline = item.Summary.Bullets[0]
	}

	labels := ""
	if len(item.Labels) > 0 {
		parts := make([]string, len(item.Labels))
		for i, l := range item.Labels {
			parts[i] = "`" + l + "`"
		}
		labels = " " + strings.Join(parts, " ")
	}

	fmt.Fprintf(w, "### [%d] %s — %s\n\n", item.Score, item.Post.Channel, headline)

	if labels != "" {
		fmt.Fprintf(w, "Labels:%s\n\n", labels)
	}

	for _, bullet := range item.Summary.Bullets[1:] {
		fmt.Fprintf(w, "- %s\n", bullet)
	}
	if len(item.Summary.Bullets) > 1 {
		fmt.Fprintln(w)
	}

	if len(item.AlsoIn) > 0 {
		fmt.Fprintf(w, "Also in: %s\n\n", strings.Join(item.AlsoIn, ", "))
	}

	if item.Post.URL != "" {
		fmt.Fprintf(w, "[Link](%s)\n\n", item.Post.URL)
	}
}

func (f *MarkdownFormatter) writeSkimItem(w io.Writer, item DigestItem) {
	headline := ""
	if len(item.Summary.Bullets) > 0 {
		headline = item.Summary.Bullets[0]
	}

	fmt.Fprintf(w, "- **[%d]** %s — %s", item.Score, item.Post.Channel, headline)
	if len(item.AlsoIn) > 0 {
		fmt.Fprintf(w, " _(also in: %s)_", strings.Join(item.AlsoIn, ", "))
	}
	fmt.Fprintln(w)
}
