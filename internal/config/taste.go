package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type TasteProfile struct {
	Weights    Weights             `yaml:"weights"`
	Labels     map[string][]string `yaml:"labels"`
	Rules      []Rule              `yaml:"rules"`
	Thresholds Thresholds          `yaml:"thresholds"`
}

type Weights struct {
	HighSignal map[string]int `yaml:"high_signal"`
	LowSignal  map[string]int `yaml:"low_signal"`
}

type Rule struct {
	If   RuleCondition `yaml:"if"`
	Then RuleAction    `yaml:"then"`
}

type RuleCondition struct {
	ContainsAny []string `yaml:"contains_any"`
}

type RuleAction struct {
	ScoreAdd int      `yaml:"score_add"`
	Labels   []string `yaml:"labels"`
}

type Thresholds struct {
	ReadNow int `yaml:"read_now"`
	Skim    int `yaml:"skim"`
	Ignore  int `yaml:"ignore"`
}

// LoadTaste reads a taste profile YAML file and validates it.
func LoadTaste(path string) (*TasteProfile, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("taste profile path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read taste profile: %w", err)
	}

	var tp TasteProfile
	if err := yaml.Unmarshal(data, &tp); err != nil {
		return nil, fmt.Errorf("parse taste profile: %w", err)
	}

	if err := validateTaste(&tp); err != nil {
		return nil, fmt.Errorf("validate taste profile: %w", err)
	}

	return &tp, nil
}

func validateTaste(tp *TasteProfile) error {
	if tp.Thresholds.ReadNow <= tp.Thresholds.Skim {
		return fmt.Errorf("thresholds: read_now (%d) must be greater than skim (%d)",
			tp.Thresholds.ReadNow, tp.Thresholds.Skim)
	}
	if tp.Thresholds.Skim <= tp.Thresholds.Ignore {
		return fmt.Errorf("thresholds: skim (%d) must be greater than ignore (%d)",
			tp.Thresholds.Skim, tp.Thresholds.Ignore)
	}
	return nil
}
