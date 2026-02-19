// generate-reference generates REFERENCE.md from Cobra command metadata.
//
// It reads REFERENCE.md (or --template), finds GENERATE marker pairs, and replaces
// content between START and END with generated output. Structure is controlled by
// the template; see REFERENCE.template.md.
//
// Marker format:
//   - GENERATE_TOC:START / GENERATE_TOC:END - table of contents
//   - GENERATE_GLOBAL_FLAGS:START / GENERATE_GLOBAL_FLAGS:END - global options
//   - GENERATE:path1|path2|path3:START / GENERATE:path1|path2|path3:END - command docs (| separator)
//
// Usage:
//
//	cp REFERENCE.template.md REFERENCE.md && go run ./tools/generate-reference
//	go run ./tools/generate-reference --check
//	go run ./tools/generate-reference --output docs/REFERENCE.md
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func main() {
	check := flag.Bool("check", false, "generate to temp file and diff against REFERENCE.md; exit 1 if different")
	output := flag.String("output", "REFERENCE.md", "output file path")
	template := flag.String("template", "", "template file (default: same as output)")
	flag.Parse()

	if *template == "" {
		*template = *output
	}

	root := cmd.RootCmd()
	content, err := generateFromTemplate(root, *template)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}

	if *check {
		existing, err := os.ReadFile(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", *output, err)
			os.Exit(1)
		}
		if !bytes.Equal(existing, content) {
			runCheck(*output, content)
			os.Exit(1)
		}
		fmt.Println("REFERENCE.md is up to date")
		return
	}

	if err := os.WriteFile(*output, content, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *output, err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s\n", *output)
}

// generateMarker matches <!-- GENERATE_XXX:START --> or <!-- GENERATE:paths:START --> etc.
// _[A-Z0-9_]+ matches _TOC, _GLOBAL_FLAGS; :[^:]+ matches :path1|path2|path3
var generateMarkerRE = regexp.MustCompile(`(?m)^(<!-- (GENERATE(?:_[A-Z0-9_]+|:[^:]+):)(START|END) -->)\s*$`)

func generateFromTemplate(root *cobra.Command, templatePath string) ([]byte, error) {
	input, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}
	s := string(input)

	// Replace GENERATE_TIMESTAMP
	ts := fmt.Sprintf("<!-- generated at %s -->", time.Now().Format("2006-01-02"))
	s = strings.Replace(s, "<!-- GENERATE_TIMESTAMP -->", ts, 1)

	// Find and replace each GENERATE block
	pos := 0
	for {
		idx := generateMarkerRE.FindStringIndex(s[pos:])
		if idx == nil {
			break
		}
		startPos := pos + idx[0]
		sub := generateMarkerRE.FindStringSubmatch(s[pos:])
		fullMatch := sub[1]
		markerID := sub[2]   // e.g. "GENERATE_TOC" or "GENERATE:login|logout|whoami"
		startOrEnd := sub[3] // "START" or "END"

		if startOrEnd != "START" {
			pos = startPos + len(fullMatch)
			continue
		}

		// Find matching END marker
		afterStart := s[startPos+len(fullMatch):]
		endIdx := generateMarkerRE.FindStringIndex(afterStart)
		if endIdx == nil {
			pos = startPos + len(fullMatch)
			continue
		}
		endSub := generateMarkerRE.FindStringSubmatch(afterStart)
		if endSub[3] != "END" || endSub[2] != markerID {
			pos = startPos + len(fullMatch)
			continue
		}

		generated := generateBlockContent(root, markerID)
		replacement := fullMatch + "\n" + generated + "\n" + endSub[1]
		s = s[:startPos] + replacement + afterStart[endIdx[1]:]
		pos = startPos + len(replacement)
	}

	return []byte(s), nil
}

func generateBlockContent(root *cobra.Command, markerID string) string {
	// markerID has trailing colon, e.g. "GENERATE_TOC:" or "GENERATE:paths:"
	id := strings.TrimSuffix(markerID, ":")
	switch id {
	case "GENERATE_TOC":
		return generateTOC(root)
	case "GENERATE_GLOBAL_FLAGS":
		return generateGlobalFlags(root)
	default:
		if strings.HasPrefix(id, "GENERATE:") {
			paths := strings.Split(strings.TrimPrefix(id, "GENERATE:"), "|")
			return generateCommands(root, paths)
		}
		return ""
	}
}

func generateTOC(root *cobra.Command) string {
	// Groups only; no per-command sub-links
	sections := []string{
		"Global Options", "Authentication", "Projects", "Local Development", "Gateway",
		"Connections", "Sources", "Destinations", "Transformations", "Events", "Requests",
		"Attempts", "Utilities",
	}
	var b bytes.Buffer
	for _, title := range sections {
		anchor := headingToAnchor(title)
		b.WriteString(fmt.Sprintf("- [%s](#%s)\n", title, anchor))
	}
	return strings.TrimRight(b.String(), "\n")
}

func generateGlobalFlags(root *cobra.Command) string {
	type flagInfo struct {
		name, shorthand, ftype, usage string
	}
	var flags []flagInfo
	seen := make(map[string]bool)
	collect := func(f *pflag.Flag) {
		if f.Hidden || seen[f.Name] {
			return
		}
		seen[f.Name] = true
		usage := f.Usage
		if f.DefValue != "" && f.DefValue != "false" {
			usage += fmt.Sprintf(" (default %q)", f.DefValue)
		}
		flags = append(flags, flagInfo{f.Name, f.Shorthand, f.Value.Type(), usage})
	}
	root.PersistentFlags().VisitAll(collect)
	root.Flags().VisitAll(collect)

	var b bytes.Buffer
	b.WriteString("| Flag | Type | Description |\n")
	b.WriteString("|------|------|-------------|\n")
	for _, f := range flags {
		var flag string
		if f.shorthand != "" {
			flag = fmt.Sprintf("`-%s, --%s`", f.shorthand, f.name)
		} else {
			flag = fmt.Sprintf("`--%s`", f.name)
		}
		usage := strings.ReplaceAll(f.usage, "|", "\\|")
		b.WriteString(fmt.Sprintf("| %s | `%s` | %s |\n", flag, f.ftype, usage))
	}
	return b.String()
}

func generateCommands(root *cobra.Command, paths []string) string {
	// Resolve commands and build in-page TOC + content
	type item struct {
		cmd  *cobra.Command
		path string
	}
	var items []item
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		parts := strings.Fields(path)
		c, _, err := root.Find(parts)
		if err != nil || c == root {
			continue
		}
		if c.HasSubCommands() && path != "gateway" {
			continue
		}
		items = append(items, item{c, path})
	}
	if len(items) == 0 {
		return ""
	}

	var b bytes.Buffer
	// In-page TOC for subcommands in this section
	if len(items) > 1 {
		b.WriteString("In this section:\n\n")
		for _, it := range items {
			anchor := headingToAnchor(it.cmd.CommandPath())
			b.WriteString(fmt.Sprintf("- [%s](#%s)\n", it.cmd.CommandPath(), anchor))
		}
		b.WriteString("\n")
	}

	for _, it := range items {
		section := commandSection(it.cmd)
		if section != "" {
			b.WriteString(section)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func commandSection(c *cobra.Command) string {
	var b bytes.Buffer
	b.WriteString("### " + c.CommandPath() + "\n\n")

	desc := c.Short
	if c.Long != "" {
		desc = strings.TrimSpace(c.Long)
	}
	// Detect "Examples:" block in Long and format it as code so # lines aren't interpreted as headings
	desc, examplesBlock := extractExamplesFromLong(desc)
	if desc != "" {
		b.WriteString(desc + "\n\n")
	}

	if c.Use != "" {
		b.WriteString("**Usage:**\n\n```bash\n")
		b.WriteString(c.UseLine() + "\n")
		b.WriteString("```\n\n")
	}

	// Show Examples: from Long (formatted in code block) and/or Cobra Example
	// Both can be present—Long has brief contextual examples, Example has in-depth ones
	if examplesBlock != "" || c.Example != "" {
		b.WriteString("**Examples:**\n\n```bash\n")
		if examplesBlock != "" {
			b.WriteString(examplesBlock)
		}
		if examplesBlock != "" && c.Example != "" {
			b.WriteString("\n\n")
		}
		if c.Example != "" {
			b.WriteString(normalizeIndent(strings.TrimSpace(c.Example)))
		}
		b.WriteString("\n```\n\n")
	}

	var flagRows []struct {
		flag, ftype, usage string
	}
	collectFlag := func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" || f.Name == "version" {
			return
		}
		var flag string
		if f.Shorthand != "" {
			flag = fmt.Sprintf("`-%s, --%s`", f.Shorthand, f.Name)
		} else {
			flag = fmt.Sprintf("`--%s`", f.Name)
		}
		usage := f.Usage
		if f.DefValue != "" && f.DefValue != "false" {
			usage += fmt.Sprintf(" (default %q)", f.DefValue)
		}
		flagRows = append(flagRows, struct{ flag, ftype, usage string }{flag, "`" + f.Value.Type() + "`", usage})
	}
	c.Flags().VisitAll(collectFlag)
	c.InheritedFlags().VisitAll(collectFlag)
	if len(flagRows) > 0 {
		b.WriteString("**Flags:**\n\n")
		b.WriteString("| Flag | Type | Description |\n")
		b.WriteString("|------|------|-------------|\n")
		for _, r := range flagRows {
			// Escape | in description for table
			usage := strings.ReplaceAll(r.usage, "|", "\\|")
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.flag, r.ftype, usage))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// extractExamplesFromLong finds an "Examples:" block in Long text and returns
// (prose, examplesBlock). The examples block is formatted in a code block by
// the caller so lines like "# comment" render as code, not markdown headings.
// Normalizes indentation by stripping the minimum common indent so all lines
// align consistently.
func extractExamplesFromLong(long string) (prose, examplesBlock string) {
	idx := strings.Index(long, "Examples:")
	if idx < 0 {
		return long, ""
	}
	prose = strings.TrimSpace(long[:idx])
	afterLabel := long[idx+len("Examples:"):]
	afterLabel = strings.TrimPrefix(afterLabel, "\n")
	afterLabel = strings.TrimPrefix(afterLabel, "\r\n")
	block := strings.ReplaceAll(afterLabel, "\t", "  ")
	examplesBlock = strings.TrimSpace(normalizeIndent(block))
	return prose, examplesBlock
}

// normalizeIndent strips leading whitespace from each line so all lines are
// consistently left-aligned in the output.
func normalizeIndent(block string) string {
	lines := strings.Split(block, "\n")
	var out []string
	for _, line := range lines {
		out = append(out, strings.TrimLeft(line, " \t"))
	}
	return strings.Join(out, "\n")
}

func headingToAnchor(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(s, "")
}

func runCheck(refPath string, generated []byte) {
	tmp, err := os.CreateTemp("", "reference-*.md")
	if err != nil {
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	tmp.Write(generated)
	tmp.Close()
	absRef, _ := filepath.Abs(refPath)
	fmt.Fprintf(os.Stderr, "REFERENCE.md is out of date. Run: go run ./tools/generate-reference\n")
	fmt.Fprintf(os.Stderr, "Diff: diff %s %s\n", absRef, tmpPath)
}
