package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Defaults struct {
	Timezone       string `json:"timezone"`
	Agent          string `json:"agent"`
	TimeoutMinutes int    `json:"timeout_minutes"`
}

type Schedule struct {
	Days []string `json:"days"`
	Time string   `json:"time"`
}

type Newsletter struct {
	Slug           string   `json:"slug"`
	Enabled        *bool    `json:"enabled"`
	Prompt         string   `json:"prompt"`
	Agent          string   `json:"agent"`
	TimeoutMinutes int      `json:"timeout_minutes"`
	Schedule       Schedule `json:"schedule"`
	PromptPath     string   `json:"-"`
	CronWeekdays   []string `json:"-"`
}

type Configuration struct {
	Defaults    Defaults
	Newsletters []Newsletter
}

func Load(configDir string) (Configuration, error) {
	var config Configuration
	if err := decodeFile(filepath.Join(configDir, "defaults.json"), &config.Defaults); err != nil {
		return config, fmt.Errorf("load defaults: %w", err)
	}
	if err := validateDefaults(config.Defaults); err != nil {
		return config, err
	}
	if _, err := time.LoadLocation(config.Defaults.Timezone); err != nil {
		return config, fmt.Errorf("invalid timezone %q: %w", config.Defaults.Timezone, err)
	}

	entries, err := os.ReadDir(filepath.Join(configDir, "newsletters"))
	if err != nil {
		return config, fmt.Errorf("read newsletter configs: %w", err)
	}
	promptsDir := filepath.Join(configDir, "..", "prompts")
	configuredSlugs := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(configDir, "newsletters", entry.Name())
		var newsletter Newsletter
		if err := decodeFile(path, &newsletter); err != nil {
			return config, fmt.Errorf("load %s: %w", entry.Name(), err)
		}
		if err := validateNewsletter(&newsletter, entry.Name(), promptsDir, config.Defaults); err != nil {
			return config, fmt.Errorf("validate %s: %w", entry.Name(), err)
		}
		configuredSlugs[newsletter.Slug] = true
		config.Newsletters = append(config.Newsletters, newsletter)
	}
	if len(config.Newsletters) == 0 {
		return config, fmt.Errorf("no newsletter JSON files found in %s", filepath.Join(configDir, "newsletters"))
	}
	promptEntries, err := os.ReadDir(promptsDir)
	if err != nil {
		return config, fmt.Errorf("read prompts: %w", err)
	}
	for _, entry := range promptEntries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		slug := strings.TrimSuffix(entry.Name(), ".md")
		if !configuredSlugs[slug] {
			return config, fmt.Errorf("prompt %s has no matching config/newsletters/%s.json", entry.Name(), slug)
		}
	}
	return config, nil
}

func (c Configuration) RenderCrontab() string {
	lines := []string{"CRON_TZ=" + c.Defaults.Timezone}
	for _, newsletter := range c.Newsletters {
		if !*newsletter.Enabled {
			continue
		}
		clock, _ := time.Parse("15:04", newsletter.Schedule.Time)
		lines = append(lines, fmt.Sprintf("%d %d * * %s newsletter run %s", clock.Minute(), clock.Hour(), strings.Join(newsletter.CronWeekdays, ","), newsletter.Slug))
	}
	return strings.Join(lines, "\n") + "\n"
}

func decodeFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("contains more than one JSON value")
		}
		return err
	}
	return nil
}

func validateDefaults(defaults Defaults) error {
	if strings.TrimSpace(defaults.Timezone) == "" {
		return fmt.Errorf("timezone is required")
	}
	if err := validateAgent(defaults.Agent); err != nil {
		return fmt.Errorf("default agent: %w", err)
	}
	if defaults.TimeoutMinutes < 1 {
		return fmt.Errorf("timeout_minutes must be positive")
	}
	return nil
}

func validateNewsletter(newsletter *Newsletter, filename, promptsDir string, defaults Defaults) error {
	if !slugPattern.MatchString(newsletter.Slug) {
		return fmt.Errorf("invalid slug %q", newsletter.Slug)
	}
	if filename != newsletter.Slug+".json" {
		return fmt.Errorf("filename must be %s.json", newsletter.Slug)
	}
	if newsletter.Enabled == nil {
		return fmt.Errorf("enabled is required")
	}
	if newsletter.Prompt != "prompts/"+newsletter.Slug+".md" {
		return fmt.Errorf("prompt must be prompts/%s.md", newsletter.Slug)
	}
	promptPath := filepath.Join(promptsDir, newsletter.Slug+".md")
	info, err := os.Lstat(promptPath)
	if err != nil {
		return fmt.Errorf("prompt file %s: %w", newsletter.Prompt, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("prompt file %s must be a regular file", newsletter.Prompt)
	}
	newsletter.PromptPath = promptPath
	if newsletter.Agent == "" {
		newsletter.Agent = defaults.Agent
	}
	if err := validateAgent(newsletter.Agent); err != nil {
		return err
	}
	if newsletter.TimeoutMinutes == 0 {
		newsletter.TimeoutMinutes = defaults.TimeoutMinutes
	} else if newsletter.TimeoutMinutes < 1 {
		return fmt.Errorf("timeout_minutes must be positive")
	}
	return validateSchedule(newsletter)
}

func validateAgent(agent string) error {
	if agent != "codex" && agent != "claude" {
		return fmt.Errorf("agent must be codex or claude")
	}
	return nil
}

func validateSchedule(newsletter *Newsletter) error {
	clock, err := time.Parse("15:04", newsletter.Schedule.Time)
	if err != nil || clock.Format("15:04") != newsletter.Schedule.Time {
		return fmt.Errorf("schedule.time must use HH:MM 24-hour format")
	}
	if len(newsletter.Schedule.Days) == 0 {
		return fmt.Errorf("schedule.days must not be empty")
	}
	weekdays := map[string]string{
		"sunday": "0", "monday": "1", "tuesday": "2", "wednesday": "3",
		"thursday": "4", "friday": "5", "saturday": "6",
	}
	seen := make(map[string]bool, len(newsletter.Schedule.Days))
	for _, day := range newsletter.Schedule.Days {
		cronDay, ok := weekdays[day]
		if !ok {
			return fmt.Errorf("invalid schedule day %q", day)
		}
		if seen[day] {
			return fmt.Errorf("duplicate schedule day %q", day)
		}
		seen[day] = true
		newsletter.CronWeekdays = append(newsletter.CronWeekdays, cronDay)
	}
	sort.Slice(newsletter.CronWeekdays, func(i, j int) bool {
		return newsletter.CronWeekdays[i] < newsletter.CronWeekdays[j]
	})
	return nil
}
