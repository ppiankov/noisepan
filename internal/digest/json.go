package digest

import (
	"encoding/json"
	"io"
)

type jsonDigest struct {
	Meta    jsonMeta   `json:"meta"`
	ReadNow []jsonItem `json:"read_now"`
	Skims   []jsonItem `json:"skims"`
	Ignored int        `json:"ignored"`
}

type jsonMeta struct {
	Channels   int    `json:"channels"`
	TotalPosts int    `json:"total_posts"`
	Since      string `json:"since"`
}

type jsonItem struct {
	Source   string   `json:"source"`
	Channel  string   `json:"channel"`
	URL      string   `json:"url,omitempty"`
	PostedAt string   `json:"posted_at"`
	Score    int      `json:"score"`
	Tier     string   `json:"tier"`
	Labels   []string `json:"labels,omitempty"`
	Headline string   `json:"headline"`
	Bullets  []string `json:"bullets,omitempty"`
	AlsoIn   []string `json:"also_in,omitempty"`
}

// JSONFormatter formats a digest as JSON.
type JSONFormatter struct{}

// NewJSON creates a JSON formatter.
func NewJSON() *JSONFormatter {
	return &JSONFormatter{}
}

// Format writes the digest as JSON to w.
func (f *JSONFormatter) Format(w io.Writer, input DigestInput) error {
	readNow, skims, ignoreCount := groupByTier(input.Items)

	out := jsonDigest{
		Meta: jsonMeta{
			Channels:   input.Channels,
			TotalPosts: input.TotalPosts,
			Since:      formatDuration(input.Since),
		},
		ReadNow: toJSONItems(readNow),
		Skims:   toJSONItems(skims),
		Ignored: ignoreCount,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func toJSONItems(items []DigestItem) []jsonItem {
	result := make([]jsonItem, 0, len(items))
	for _, item := range items {
		headline := ""
		if len(item.Summary.Bullets) > 0 {
			headline = item.Summary.Bullets[0]
		}

		ji := jsonItem{
			Source:   item.Post.Source,
			Channel:  item.Post.Channel,
			URL:      item.Post.URL,
			PostedAt: item.Post.PostedAt.Format("2006-01-02T15:04:05Z"),
			Score:    item.Score,
			Tier:     item.Tier,
			Labels:   item.Labels,
			Headline: headline,
			Bullets:  item.Summary.Bullets[1:],
			AlsoIn:   item.AlsoIn,
		}
		if len(ji.Bullets) == 0 {
			ji.Bullets = nil
		}
		result = append(result, ji)
	}
	return result
}
