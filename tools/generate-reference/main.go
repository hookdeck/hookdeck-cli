// generate-reference generates REFERENCE.md (or other files) from Cobra command metadata.
//
// It reads input file(s), finds GENERATE marker pairs, and replaces content between
// START and END with generated output. Structure is controlled by the input file.
//
// Marker format:
//   - GENERATE_TOC:START ... GENERATE_END - table of contents
//   - GENERATE_GLOBAL_FLAGS:START ... GENERATE_END - global options
//   - GENERATE_HELP:path:START ... GENERATE_END - help output for command (e.g. path=connection)
//   - GENERATE:path1|path2|path3:START ... GENERATE_END - command docs (| separator)
//
// Use <!-- GENERATE_END --> to close any block (no need to repeat the command list).
//
// Usage:
//
//	go run ./tools/generate-reference --input REFERENCE.md                    # in-place
//	go run ./tools/generate-reference --input REFERENCE.template.md --output REFERENCE.md
//	go run ./tools/generate-reference --input a.mdoc --input b.mdoc           # batch, each in-place
//	go run ./tools/generate-reference --input REFERENCE.md --check            # verify up to date
//
// For website two-column layout (section/div/aside), add --no-toc, --no-examples-heading, and wrappers:
//
//	--no-toc --no-examples-heading --wrapper-section-start "<section>" --wrapper-section-end "</section>"
//	--wrapper-main-start "<div>" --wrapper-main-end "</div>"
//	--wrapper-aside-start "<aside>" --wrapper-aside-end "</aside>"
//
// For REFERENCE.md (no wrappers, include in-page TOC): use --no-wrappers or omit wrapper flags.
package main

import (
	"bytes"
	"encoding/json"
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

type inputFiles []string

func (i *inputFiles) String() string { return strings.Join(*i, ", ") }

func (i *inputFiles) Set(v string) error {
	*i = append(*i, v)
	return nil
}

// wrapConfig holds optional wrappers for layout (e.g. section/div/aside for two-column).
type wrapConfig struct {
	sectionStart, sectionEnd string
	mainStart, mainEnd       string // wraps description, usage, flags
	asideStart, asideEnd     string // wraps examples
}

// genConfig holds generation options (wrappers + toggles).
type genConfig struct {
	wrap              wrapConfig
	noToc             bool // omit in-page TOC (e.g. for website with sidebar nav)
	noWrappers        bool // ignore wrapper flags; use for REFERENCE.md
	noExamplesHeading bool // omit "**Examples:**" heading before examples block
}

func main() {
	check := flag.Bool("check", false, "generate to temp and diff; exit 1 if different")
	output := flag.String("output", "", "output file (optional; if omitted, write to input)")
	noToc := flag.Bool("no-toc", false, "omit in-page table of contents (for website)")
	noWrappers := flag.Bool("no-wrappers", false, "do not use section/div/aside wrappers (for REFERENCE.md)")
	noExamplesHeading := flag.Bool("no-examples-heading", false, "omit Examples heading before examples block")
	var inputs inputFiles
	wrap := wrapConfig{}
	flag.Var(&inputs, "input", "input file (required, repeatable for batch)")
	flag.StringVar(&wrap.sectionStart, "wrapper-section-start", "", "wrap each command block start (e.g. <section>)")
	flag.StringVar(&wrap.sectionEnd, "wrapper-section-end", "", "wrap each command block end (e.g. </section>)")
	flag.StringVar(&wrap.mainStart, "wrapper-main-start", "", "wrap main content start (e.g. <div>)")
	flag.StringVar(&wrap.mainEnd, "wrapper-main-end", "", "wrap main content end (e.g. </div>)")
	flag.StringVar(&wrap.asideStart, "wrapper-aside-start", "", "wrap examples start (e.g. <aside>)")
	flag.StringVar(&wrap.asideEnd, "wrapper-aside-end", "", "wrap examples end (e.g. </aside>)")
	flag.Parse()

	cfg := genConfig{wrap: wrap, noToc: *noToc, noWrappers: *noWrappers, noExamplesHeading: *noExamplesHeading}
	if cfg.noWrappers {
		cfg.wrap = wrapConfig{}
	}

	if len(inputs) == 0 {
		fmt.Fprintf(os.Stderr, "generate-reference: --input is required (use --input <file>)\n")
		os.Exit(1)
	}
	if len(inputs) > 1 && *output != "" {
		fmt.Fprintf(os.Stderr, "generate-reference: cannot use --output with multiple --input (batch is in-place only)\n")
		os.Exit(1)
	}

	root := cmd.RootCmd()

	for _, inPath := range inputs {
		outPath := inPath
		if len(inputs) == 1 && *output != "" {
			outPath = *output
		}

		content, err := generateFromTemplate(root, inPath, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate: %v\n", err)
			os.Exit(1)
		}

		if *check {
			existing, err := os.ReadFile(outPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "read %s: %v\n", outPath, err)
				os.Exit(1)
			}
			if !bytes.Equal(existing, content) {
				runCheck(outPath, content)
				os.Exit(1)
			}
			fmt.Printf("%s is up to date\n", outPath)
			continue
		}

		if err := os.WriteFile(outPath, content, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", outPath, err)
			os.Exit(1)
		}
		fmt.Printf("Wrote %s\n", outPath)
	}
}

// generateMarker matches <!-- GENERATE_XXX:START --> or <!-- GENERATE:paths:START --> etc.
// _[A-Z0-9_]+ matches _TOC, _GLOBAL_FLAGS; _HELP:[^:]+ matches _HELP:connection; :[^:]+ matches :path1|path2
var generateMarkerRE = regexp.MustCompile(`(?m)^(<!-- (GENERATE(?:_[A-Z0-9_]+|_HELP:[^:]+|:[^:]+):)(START|END) -->)\s*$`)

// generateEndMarker is the simple closing tag (no command list required).
const generateEndMarker = "<!-- GENERATE_END -->"
var generateEndRE = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(generateEndMarker) + `\s*$`)

// findNextEndMarker returns (start, length) of the next GENERATE*:END marker, or (-1, 0).
func findNextEndMarker(s string) (start, length int) {
	idx := generateMarkerRE.FindStringIndex(s)
	if idx == nil {
		return -1, 0
	}
	sub := generateMarkerRE.FindStringSubmatch(s)
	if sub[3] != "END" {
		return -1, 0
	}
	return idx[0], idx[1] - idx[0]
}

// pickFirstMatch returns (start, length) of whichever match appears first. (-1, 0) means no match.
func pickFirstMatch(simpleIdx []int, legacyStart, legacyLen int) (start, length int) {
	legacyValid := legacyStart >= 0
	switch {
	case simpleIdx != nil && !legacyValid:
		return simpleIdx[0], simpleIdx[1] - simpleIdx[0]
	case simpleIdx == nil && legacyValid:
		return legacyStart, legacyLen
	case simpleIdx != nil && legacyValid:
		if simpleIdx[0] <= legacyStart {
			return simpleIdx[0], simpleIdx[1] - simpleIdx[0]
		}
		return legacyStart, legacyLen
	default:
		return -1, 0
	}
}

func generateFromTemplate(root *cobra.Command, templatePath string, cfg genConfig) ([]byte, error) {
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

		// Find the next END marker (simple GENERATE_END or legacy GENERATE_*:END)
		afterStart := s[startPos+len(fullMatch):]
		simpleIdx := generateEndRE.FindStringIndex(afterStart)
		legacyStart, legacyLen := findNextEndMarker(afterStart)
		endAt, endLen := pickFirstMatch(simpleIdx, legacyStart, legacyLen)
		if endAt < 0 {
			pos = startPos + len(fullMatch)
			continue
		}

		generated := generateBlockContent(root, markerID, cfg)
		replacement := fullMatch + "\n" + generated + "\n" + generateEndMarker
		s = s[:startPos] + replacement + afterStart[endAt+endLen:]
		pos = startPos + len(replacement)
	}

	return []byte(s), nil
}

func generateBlockContent(root *cobra.Command, markerID string, cfg genConfig) string {
	// markerID has trailing colon, e.g. "GENERATE_TOC:" or "GENERATE:paths:"
	id := strings.TrimSuffix(markerID, ":")
	wrap := cfg.wrap
	switch id {
	case "GENERATE_TOC":
		return generateTOC(root)
	case "GENERATE_GLOBAL_FLAGS":
		return generateGlobalFlags(root)
	default:
		if strings.HasPrefix(id, "GENERATE_HELP:") {
			path := strings.TrimPrefix(id, "GENERATE_HELP:")
			return generateHelpOutput(root, path, wrap)
		}
		if strings.HasPrefix(id, "GENERATE:") {
			paths := strings.Split(strings.TrimPrefix(id, "GENERATE:"), "|")
			return generateCommands(root, paths, cfg)
		}
		return ""
	}
}

// generateHelpOutput returns structured help for a command group: description, usage, global flags,
// then examples. Order matches commandSection (flags before examples). Applies wrap config when set.
func generateHelpOutput(root *cobra.Command, path string, wrap wrapConfig) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	parts := strings.Fields(path)
	c, _, err := root.Find(parts)
	if err != nil || c == root {
		return ""
	}

	// Main: section heading (## GroupName) then description, usage, global flags
	var mainBuf bytes.Buffer
	if wrap.sectionStart != "" {
		if name := c.Name(); len(name) > 0 {
			mainBuf.WriteString("## " + strings.ToUpper(name[:1]) + name[1:] + "\n\n")
		}
	}
	desc := c.Long
	if desc == "" {
		desc = c.Short
	}
	desc, _ = extractExamplesFromLong(strings.TrimSpace(desc))
	if desc != "" {
		mainBuf.WriteString(wrapFlagsInBackticks(desc) + "\n\n")
	}
	if c.Use != "" {
		mainBuf.WriteString("**Usage:**\n\n```bash\n")
		usage := c.UseLine()
		if c.HasAvailableSubCommands() && !strings.Contains(usage, "[command]") {
			usage += " [command]"
		}
		mainBuf.WriteString(usage + "\n")
		mainBuf.WriteString("```\n\n")
	}
	mainBuf.WriteString(globalFlagsTable(root))
	mainStr := strings.TrimRight(mainBuf.String(), "\n")

	// Examples: available commands (comes after flags, no heading to avoid layout issues)
	var examplesBuf bytes.Buffer
	if c.HasAvailableSubCommands() {
		examplesBuf.WriteString("```bash\n")
		for _, sub := range c.Commands() {
			if sub.Hidden {
				continue
			}
			cmdPath := sub.CommandPath()
			examplesBuf.WriteString(cmdPath)
			if sub.Short != "" {
				examplesBuf.WriteString("  # " + sub.Short)
			}
			examplesBuf.WriteString("\n")
		}
		examplesBuf.WriteString("```\n")
	}
	examplesStr := strings.TrimSpace(examplesBuf.String())

	// Apply wrappers (match project/listen: section > div with blank line, aside)
	var out bytes.Buffer
	if wrap.sectionStart != "" {
		out.WriteString(wrap.sectionStart + "\n")
	}
	if wrap.mainStart != "" {
		out.WriteString(wrap.mainStart + "\n\n")
	}
	out.WriteString(mainStr)
	if wrap.mainEnd != "" {
		out.WriteString("\n" + wrap.mainEnd + "\n")
	}
	if wrap.asideStart != "" && examplesStr != "" {
		out.WriteString(wrap.asideStart + "\n\n")
	}
	out.WriteString(examplesStr)
	if wrap.asideEnd != "" && examplesStr != "" {
		out.WriteString("\n" + wrap.asideEnd + "\n")
	}
	if wrap.sectionEnd != "" {
		out.WriteString(wrap.sectionEnd + "\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

// globalFlagsTable returns root-level flags as a markdown table for command-group docs.
func globalFlagsTable(root *cobra.Command) string {
	type flagInfo struct {
		name, shorthand, ftype, usage string
	}
	var flags []flagInfo
	seen := make(map[string]bool)
	collect := func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" || f.Name == "version" || seen[f.Name] {
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
	if len(flags) == 0 {
		return ""
	}
	var b bytes.Buffer
	b.WriteString("**Global options:**\n\n")
	b.WriteString("| Flag | Type | Description |\n")
	b.WriteString("|------|------|-------------|\n")
	for _, f := range flags {
		var flag string
		if f.shorthand != "" {
			flag = fmt.Sprintf("`-%s, --%s`", f.shorthand, f.name)
		} else {
			flag = fmt.Sprintf("`--%s`", f.name)
		}
		usage := normalizeUsageForTable(f.usage)
		usage = strings.ReplaceAll(usage, "|", "\\|")
		b.WriteString(fmt.Sprintf("| %s | `%s` | %s |\n", flag, f.ftype, usage))
	}
	return b.String()
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
		usage := normalizeUsageForTable(f.usage)
		usage = strings.ReplaceAll(usage, "|", "\\|")
		b.WriteString(fmt.Sprintf("| %s | `%s` | %s |\n", flag, f.ftype, usage))
	}
	return b.String()
}

// rootFlagNames returns the set of non-hidden flag names defined on the root (persistent + local).
// Hidden flags (e.g. root's --api-key) are excluded so that commands that define their own
// visible version (e.g. ci's --api-key) will include it in their per-command flag table.
func rootFlagNames(root *cobra.Command) map[string]bool {
	names := make(map[string]bool)
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			names[f.Name] = true
		}
	})
	root.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			names[f.Name] = true
		}
	})
	return names
}

func generateCommands(root *cobra.Command, paths []string, cfg genConfig) string {
	globalFlagNames := rootFlagNames(root)
	wrap := cfg.wrap

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
	// In-page TOC (optional; omit for website which has sidebar nav)
	if !cfg.noToc && len(items) > 1 {
		tocContent := ""
		for _, it := range items {
			label, anchor := commandHeadingLabelAndAnchor(root, it.cmd)
			tocContent += fmt.Sprintf("- [%s](#%s)\n", label, anchor)
		}
		tocContent = strings.TrimSuffix(tocContent, "\n")
		if wrap.sectionStart != "" && wrap.mainStart != "" {
			b.WriteString(wrap.sectionStart + "\n")
			b.WriteString(wrap.mainStart + "\n")
			b.WriteString(tocContent + "\n")
			b.WriteString(wrap.mainEnd + "\n")
			b.WriteString(wrap.sectionEnd + "\n")
		} else {
			b.WriteString(tocContent + "\n\n")
		}
	}

	for _, it := range items {
		section := commandSection(root, it.cmd, wrap, globalFlagNames, cfg.noExamplesHeading)
		if section != "" {
			b.WriteString(section)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// commandHeadingLabelAndAnchor returns the display label and anchor slug for a command.
// Root-level commands use title case (e.g. "CI", "Listen") and command name as anchor.
func commandHeadingLabelAndAnchor(root *cobra.Command, c *cobra.Command) (label, anchor string) {
	if c.Parent() == root && len(c.Name()) > 0 {
		title := c.Name()
		if strings.EqualFold(title, "ci") {
			title = "CI"
		} else if len(title) > 1 {
			title = strings.ToUpper(title[:1]) + title[1:]
		} else {
			title = strings.ToUpper(title)
		}
		return title, headingToAnchor(c.Name())
	}
	return c.CommandPath(), headingToAnchor(c.CommandPath())
}

func commandSection(root *cobra.Command, c *cobra.Command, wrap wrapConfig, globalFlagNames map[string]bool, noExamplesHeading bool) string {
	// Order: description, usage, flags, examples (usage and flags before examples)

	// Main content: heading (## for root-level commands, ### for subcommands), description, usage, flags
	var mainBuf bytes.Buffer
	label, _ := commandHeadingLabelAndAnchor(root, c)
	level := "###"
	if c.Parent() == root && len(c.Name()) > 0 {
		level = "##"
	}
	mainBuf.WriteString(level + " " + label + "\n\n")

	desc := c.Short
	if c.Long != "" {
		desc = strings.TrimSpace(c.Long)
	}
	desc, examplesBlock := extractExamplesFromLong(desc)
	if desc != "" {
		mainBuf.WriteString(wrapFlagsInBackticks(desc) + "\n\n")
	}

	if c.Use != "" {
		mainBuf.WriteString("**Usage:**\n\n```bash\n")
		mainBuf.WriteString(c.UseLine() + "\n")
		mainBuf.WriteString("```\n\n")
	}

	// Arguments: from Annotations["cli.arguments"] if present (JSON array of {name, type, description, required})
	if argsTable := renderArgumentsTable(c); argsTable != "" {
		mainBuf.WriteString(argsTable)
	}

	// Flags: command-specific only (root-level flags omitted from per-command tables)
	var flagRows []struct {
		flag, ftype, usage string
	}
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" || f.Name == "version" || globalFlagNames[f.Name] {
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
	})
	if len(flagRows) > 0 {
		mainBuf.WriteString("**Flags:**\n\n")
		mainBuf.WriteString("| Flag | Type | Description |\n")
		mainBuf.WriteString("|------|------|-------------|\n")
		for _, r := range flagRows {
			usage := normalizeUsageForTable(r.usage)
			usage = wrapFlagsInBackticks(usage)
			usage = strings.ReplaceAll(usage, "|", "\\|")
			mainBuf.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.flag, r.ftype, usage))
		}
		mainBuf.WriteString("\n")
	}

	// Examples (shown last, optionally wrapped in aside)
	var examplesBuf bytes.Buffer
	if examplesBlock != "" || c.Example != "" {
		if !noExamplesHeading {
			examplesBuf.WriteString("**Examples:**\n\n")
		}
		examplesBuf.WriteString("```bash\n")
		if examplesBlock != "" {
			examplesBuf.WriteString(examplesBlock)
		}
		if examplesBlock != "" && c.Example != "" {
			examplesBuf.WriteString("\n\n")
		}
		if c.Example != "" {
			examplesBuf.WriteString(normalizeIndent(strings.TrimSpace(c.Example)))
		}
		examplesBuf.WriteString("\n```\n\n")
	}

	// Apply wrappers and assemble (match project/listen structure)
	var out bytes.Buffer
	mainStr := mainBuf.String()
	examplesStr := examplesBuf.String()

	if wrap.sectionStart != "" {
		out.WriteString(wrap.sectionStart + "\n")
	}
	if wrap.mainStart != "" {
		out.WriteString(wrap.mainStart + "\n\n")
	}
	out.WriteString(mainStr)
	if wrap.mainEnd != "" {
		out.WriteString(wrap.mainEnd + "\n")
	}
	if wrap.asideStart != "" && examplesStr != "" {
		out.WriteString(wrap.asideStart + "\n\n")
	}
	out.WriteString(examplesStr)
	if wrap.asideEnd != "" && examplesStr != "" {
		out.WriteString(wrap.asideEnd + "\n")
	}
	if wrap.sectionEnd != "" {
		out.WriteString(wrap.sectionEnd + "\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

// argSpec describes a positional argument for the CLI docs.
type argSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// renderArgumentsTable returns a markdown Arguments table if c.Annotations["cli.arguments"] contains
// valid JSON array of argSpec. Used to document positional args before the Flags table.
func renderArgumentsTable(c *cobra.Command) string {
	if c.Annotations == nil {
		return ""
	}
	raw, ok := c.Annotations["cli.arguments"]
	if !ok || raw == "" {
		return ""
	}
	var args []argSpec
	if err := json.Unmarshal([]byte(raw), &args); err != nil || len(args) == 0 {
		return ""
	}
	var b bytes.Buffer
	b.WriteString("**Arguments:**\n\n")
	b.WriteString("| Argument | Type | Description |\n")
	b.WriteString("|----------|------|-------------|\n")
	for _, a := range args {
		desc := a.Description
		if a.Required {
			desc = "**Required.** " + desc
		} else {
			desc = "**Optional.** " + desc
		}
		desc = wrapFlagsInBackticks(desc)
		desc = strings.ReplaceAll(desc, "|", "\\|")
		typ := "`" + a.Type + "`"
		if a.Type == "" {
			typ = "`string`"
		}
		argName := "`" + strings.ReplaceAll(a.Name, "`", "\\`") + "`"
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", argName, typ, desc))
	}
	b.WriteString("\n")
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

// escapeAngleBracketsForMarkdown replaces < and > with HTML entities so Markdoc and
// other parsers do not treat placeholders like <issue-id> or <number> as HTML tags.
// Use for any generator output that is embedded in markdown (usage lines, table cells).
func escapeAngleBracketsForMarkdown(s string) string {
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// normalizeUsageForTable collapses newlines and extra spaces in flag usage so markdown
// table rows stay on one line. Escapes angle brackets so Markdoc does not treat them
// as HTML tags (e.g. "<number><unit>" in granularity help). Use for any flag description
// emitted into a markdown table.
func normalizeUsageForTable(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = escapeAngleBracketsForMarkdown(s)
	return strings.TrimSpace(s)
}

// wrapFlagsInBackticks wraps flag references (--flag-name) in backticks for markdown.
// Skips segments already inside backticks to avoid double-wrapping (RE2 has no lookbehind).
var flagLongRE = regexp.MustCompile(`--([a-zA-Z][a-zA-Z0-9_-]*)`)

func wrapFlagsInBackticks(s string) string {
	parts := strings.Split(s, "`")
	for i := 0; i < len(parts); i += 2 {
		parts[i] = flagLongRE.ReplaceAllString(parts[i], "`--$1`")
	}
	return strings.Join(parts, "`")
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
	fmt.Fprintf(os.Stderr, "%s is out of date. Run: go run ./tools/generate-reference --input <template> [--output <file>]\n", refPath)
	fmt.Fprintf(os.Stderr, "Diff: diff %s %s\n", absRef, tmpPath)
}
