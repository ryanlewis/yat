package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryanlewis/yat/internal/config"
)

// InitCmd initializes a new yat project in the current directory.
type InitCmd struct {
	Dir string `arg:"" optional:"" help:"Items directory (default: spec, or prompted interactively)."`
}

type initJSON struct {
	ConfigFile string `json:"config_file"`
	Dir        string `json:"dir"`
	Created    bool   `json:"created"`
	SampleItem string `json:"sample_item,omitempty"`
}

const (
	defaultDir                      = "spec"
	sampleItemFile                  = "example.md"
	initDirPermissions  fs.FileMode = 0o750
	initFilePermissions fs.FileMode = 0o600
)

const sampleItemContent = `---
id: EXAMPLE-001
title: My first item
type: Task
priority: Medium
status: draft
---

Replace this with a description of the work to be done.
`

// initPlan captures what the init command will do.
type initPlan struct {
	dir          string
	createDir    bool
	createSample bool
	samplePath   string
	mdCount      int
}

// Run executes the init command. It takes jsonMode, stdout, and stdin directly
// because it runs before NewRunContext (the items directory may not exist yet).
func (i *InitCmd) Run(jsonMode bool, stdout io.Writer, stdin io.Reader) error {
	if existing := existingConfig(); existing != "" {
		return fmt.Errorf("yat is already initialized (%s exists)", existing)
	}

	scanner := bufio.NewScanner(stdin)

	dir := i.Dir
	if dir == "" {
		dir = promptDir(scanner, stdout)
	}

	plan := buildInitPlan(dir)
	printPlan(stdout, plan)

	fmt.Fprintf(stdout, "\nProceed? [Y/n]: ")

	if !confirmPrompt(scanner) {
		fmt.Fprintln(stdout, "Aborted.")
		return nil
	}

	if err := executePlan(plan); err != nil {
		return err
	}

	return printResult(jsonMode, stdout, plan)
}

func existingConfig() string {
	for _, name := range config.ConfigNames {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}

	return ""
}

func buildInitPlan(dir string) initPlan {
	p := initPlan{
		dir:        dir,
		samplePath: filepath.Join(dir, sampleItemFile),
	}

	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		p.createDir = true
	} else {
		p.mdCount = countMDFiles(dir)
	}

	p.createSample = p.mdCount == 0

	return p
}

func printPlan(stdout io.Writer, p initPlan) {
	if p.mdCount > 0 {
		fmt.Fprintf(stdout, "\nFound %d existing items in %s/\n", p.mdCount, p.dir)
	}

	fmt.Fprintf(stdout, "\nyat will:\n")
	fmt.Fprintf(stdout, "  create %s (dir: %s)\n", config.DefaultConfigFile, p.dir)

	if p.createDir {
		fmt.Fprintf(stdout, "  create %s/\n", p.dir)
	}

	if p.createSample {
		fmt.Fprintf(stdout, "  create %s\n", p.samplePath)
	}
}

func executePlan(p initPlan) error {
	if p.createDir {
		if err := os.MkdirAll(p.dir, initDirPermissions); err != nil {
			return fmt.Errorf("creating directory %s: %w", p.dir, err)
		}
	}

	if p.createSample {
		if err := os.WriteFile(p.samplePath, []byte(sampleItemContent), initFilePermissions); err != nil {
			return fmt.Errorf("writing sample item: %w", err)
		}
	}

	configContent := fmt.Sprintf("dir: %s\n", p.dir)

	f, err := os.OpenFile(config.DefaultConfigFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, initFilePermissions)
	if err != nil {
		return fmt.Errorf("writing %s: %w", config.DefaultConfigFile, err)
	}

	_, writeErr := f.WriteString(configContent)
	if closeErr := f.Close(); writeErr == nil {
		writeErr = closeErr
	}

	if writeErr != nil {
		return fmt.Errorf("writing %s: %w", config.DefaultConfigFile, writeErr)
	}

	return nil
}

func printResult(jsonMode bool, stdout io.Writer, p initPlan) error {
	if jsonMode {
		result := initJSON{
			ConfigFile: config.DefaultConfigFile,
			Dir:        p.dir,
			Created:    p.createDir,
		}
		if p.createSample {
			result.SampleItem = p.samplePath
		}

		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(result)
	}

	fmt.Fprintln(stdout, "\nInitialized yat project.")
	fmt.Fprintf(stdout, "  config:  %s\n", config.DefaultConfigFile)

	if p.mdCount > 0 {
		fmt.Fprintf(stdout, "  items:   %s/ (%d existing items)\n", p.dir, p.mdCount)
	} else {
		fmt.Fprintf(stdout, "  items:   %s/\n", p.dir)
	}

	if p.createSample {
		fmt.Fprintf(stdout, "  example: %s\n", p.samplePath)
	}

	return nil
}

func promptDir(scanner *bufio.Scanner, stdout io.Writer) string {
	fmt.Fprintf(stdout, "Items directory [%s]: ", defaultDir)

	if scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			return line
		}
	}

	return defaultDir
}

func confirmPrompt(scanner *bufio.Scanner) bool {
	if scanner.Scan() {
		line := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return line == "" || line == "y" || line == "yes"
	}

	return false
}

func countMDFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			count++
		}
	}

	return count
}
