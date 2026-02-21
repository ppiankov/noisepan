package cli

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var importDryRun bool

var importCmd = &cobra.Command{
	Use:   "import <file.opml>",
	Short: "Import RSS feeds from an OPML file",
	Args:  cobra.ExactArgs(1),
	RunE:  importAction,
}

func init() {
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "show what would be added without modifying config")
	rootCmd.AddCommand(importCmd)
}

type opml struct {
	Body opmlBody `xml:"body"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	XMLURL   string        `xml:"xmlUrl,attr"`
	Text     string        `xml:"text,attr"`
	Outlines []opmlOutline `xml:"outline"`
}

func importAction(_ *cobra.Command, args []string) error {
	opmlPath := args[0]

	data, err := os.ReadFile(opmlPath)
	if err != nil {
		return fmt.Errorf("read OPML: %w", err)
	}

	var doc opml
	if err := xml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse OPML: %w", err)
	}

	feedURLs := extractFeedURLs(doc.Body.Outlines)
	if len(feedURLs) == 0 {
		fmt.Println("No feed URLs found in OPML file.")
		return nil
	}

	// Load existing config to find duplicates
	cfg, err := config.Load(configDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	existing := make(map[string]bool)
	for _, f := range cfg.Sources.RSS.Feeds {
		existing[f] = true
	}

	var newFeeds []string
	skipped := 0
	for _, u := range feedURLs {
		if existing[u] {
			skipped++
			continue
		}
		newFeeds = append(newFeeds, u)
	}

	if len(newFeeds) == 0 {
		fmt.Printf("All %d feeds already present, nothing to add.\n", skipped)
		return nil
	}

	if importDryRun {
		fmt.Printf("Would add %d feeds (skipping %d duplicates):\n", len(newFeeds), skipped)
		for _, f := range newFeeds {
			fmt.Printf("  + %s\n", f)
		}
		return nil
	}

	// Merge into config.yaml using yaml.Node to preserve structure
	configPath := filepath.Join(configDir, config.DefaultConfigFile)
	if err := mergeFeeds(configPath, newFeeds); err != nil {
		return fmt.Errorf("merge feeds: %w", err)
	}

	fmt.Printf("Added %d feeds, skipped %d duplicates.\n", len(newFeeds), skipped)
	return nil
}

func extractFeedURLs(outlines []opmlOutline) []string {
	var urls []string
	for _, o := range outlines {
		u := strings.TrimSpace(o.XMLURL)
		if u != "" && (strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")) {
			urls = append(urls, u)
		}
		// Recurse into nested outlines (folders)
		urls = append(urls, extractFeedURLs(o.Outlines)...)
	}
	return urls
}

// mergeFeeds reads config.yaml as a yaml.Node tree, finds sources.rss.feeds,
// appends newFeeds, and writes back preserving structure.
func mergeFeeds(configPath string, newFeeds []string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse config YAML: %w", err)
	}

	feedsNode := findFeedsNode(&doc)
	if feedsNode == nil {
		return fmt.Errorf("could not find sources.rss.feeds in config.yaml")
	}

	for _, f := range newFeeds {
		feedsNode.Content = append(feedsNode.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: f,
			Style: yaml.DoubleQuotedStyle,
		})
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(configPath, out, 0o644)
}

// findFeedsNode walks the YAML tree to find the sequence node at sources.rss.feeds.
func findFeedsNode(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return findFeedsNode(doc.Content[0])
	}

	if doc.Kind != yaml.MappingNode {
		return nil
	}

	// Find "sources" key
	sourcesNode := findMapValue(doc, "sources")
	if sourcesNode == nil {
		return nil
	}

	// Find "rss" key
	rssNode := findMapValue(sourcesNode, "rss")
	if rssNode == nil {
		return nil
	}

	// Find "feeds" key
	return findMapValue(rssNode, "feeds")
}

func findMapValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
