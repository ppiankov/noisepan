package cli

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExtractFeedURLs(t *testing.T) {
	outlines := []opmlOutline{
		{XMLURL: "https://krebsonsecurity.com/feed/", Text: "Krebs"},
		{XMLURL: "https://www.cisa.gov/cybersecurity-advisories/all.xml", Text: "CISA"},
		{XMLURL: "", Text: "Empty"},
		{XMLURL: "ftp://invalid.com/feed", Text: "Invalid scheme"},
	}

	urls := extractFeedURLs(outlines)
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d: %v", len(urls), urls)
	}
	if urls[0] != "https://krebsonsecurity.com/feed/" {
		t.Errorf("urls[0] = %q", urls[0])
	}
}

func TestExtractFeedURLs_Nested(t *testing.T) {
	outlines := []opmlOutline{
		{
			Text: "Security",
			Outlines: []opmlOutline{
				{XMLURL: "https://krebs.com/feed/"},
				{XMLURL: "https://cisa.gov/feed/"},
			},
		},
		{
			Text: "DevOps",
			Outlines: []opmlOutline{
				{XMLURL: "https://kubernetes.io/feed.xml"},
			},
		},
	}

	urls := extractFeedURLs(outlines)
	if len(urls) != 3 {
		t.Fatalf("expected 3 URLs from nested outlines, got %d: %v", len(urls), urls)
	}
}

func TestExtractFeedURLs_Empty(t *testing.T) {
	urls := extractFeedURLs(nil)
	if len(urls) != 0 {
		t.Errorf("expected 0 URLs, got %d", len(urls))
	}
}

func TestFindFeedsNode(t *testing.T) {
	yamlContent := `sources:
  rss:
    feeds:
      - "https://example.com/feed"
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatal(err)
	}

	node := findFeedsNode(&doc)
	if node == nil {
		t.Fatal("feeds node not found")
	}
	if node.Kind != yaml.SequenceNode {
		t.Errorf("expected sequence node, got %d", node.Kind)
	}
	if len(node.Content) != 1 {
		t.Errorf("expected 1 feed, got %d", len(node.Content))
	}
}

func TestFindFeedsNode_Missing(t *testing.T) {
	yamlContent := `sources:
  telegram:
    channels: []
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatal(err)
	}

	node := findFeedsNode(&doc)
	if node != nil {
		t.Error("expected nil for config without rss.feeds")
	}
}
