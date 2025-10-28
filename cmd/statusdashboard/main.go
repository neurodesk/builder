package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type BuildStatus string

const (
	BuildStatusSucceeded BuildStatus = "success"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusUnknown   BuildStatus = "unknown"
)

type BuildResult struct {
	Name                  string
	Status                BuildStatus
	RunCommand            string
	ErrorCommand          string
	ErrorOutput           string
	LogPath               string
	LogRelative           string
	LastModified          time.Time
	BaselineProvided      bool
	BaselineStatus        BuildStatus
	BaselineReason        string
	BaselineFailureOutput string
	StatusDelta           string
}

type TemplateData struct {
	GeneratedAt  time.Time
	LogsDir      string
	Builds       []BuildResult
	HasBaseline  bool
	BaselinePath string
}

var (
	errorCommandPattern = regexp.MustCompile(`ERROR: process "(?s:(.*?))" did not complete successfully`)
	templateFuncs       = template.FuncMap{
		"statusLabel":  statusLabel,
		"statusBadge":  statusBadgeClass,
		"statusBorder": statusBorderClass,
		"deltaClass":   statusDeltaClass,
	}
	dashboardTemplate = template.Must(template.New("dashboard").Funcs(templateFuncs).Parse(dashboardTemplateHTML))
)

func main() {
	logsDir := flag.String("logs", "local/local_logs", "directory containing docker build logs")
	baselinePath := flag.String("baseline", "unpriv_build_summary.json", "optional baseline summary JSON (leave empty to disable)")
	outPath := flag.String("out", "", "write HTML output to this path (default stdout)")
	flag.Parse()

	baselineEntries, baselineLoaded, err := loadBaseline(*baselinePath)
	if err != nil {
		log.Fatalf("loading baseline: %v", err)
	}

	builds, err := collectBuilds(*logsDir, baselineEntries)
	if err != nil {
		log.Fatalf("collecting build results: %v", err)
	}

	data := TemplateData{
		GeneratedAt: time.Now(),
		LogsDir:     *logsDir,
		Builds:      builds,
		HasBaseline: baselineLoaded,
		BaselinePath: func() string {
			if baselineLoaded {
				return *baselinePath
			}
			return ""
		}(),
	}

	var buf bytes.Buffer
	if err := dashboardTemplate.Execute(&buf, data); err != nil {
		log.Fatalf("rendering dashboard: %v", err)
	}

	if *outPath == "" {
		if _, err := buf.WriteTo(os.Stdout); err != nil {
			log.Fatalf("writing to stdout: %v", err)
		}
		return
	}

	if err := os.WriteFile(*outPath, buf.Bytes(), 0o644); err != nil {
		log.Fatalf("writing output file: %v", err)
	}
}

func collectBuilds(logsDir string, baseline map[string]baselineEntry) ([]BuildResult, error) {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("logs directory %q not found", logsDir)
		}
		return nil, fmt.Errorf("reading logs directory %q: %w", logsDir, err)
	}

	var builds []BuildResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".log" {
			continue
		}

		fullPath := filepath.Join(logsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", fullPath, err)
		}

		result, err := parseLog(fullPath)
		if err != nil {
			return nil, fmt.Errorf("parsing %q: %w", fullPath, err)
		}

		result.LastModified = info.ModTime()
		result.LogPath = fullPath
		if rel, err := filepath.Rel(logsDir, fullPath); err == nil {
			result.LogRelative = rel
		} else {
			result.LogRelative = entry.Name()
		}

		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		result.Name = strings.TrimPrefix(base, "build_")

		if entry, ok := baseline[normalizeRecipeName(result.Name)]; ok {
			result.BaselineProvided = true
			result.BaselineStatus = normalizeBaselineStatus(entry.Status)
			result.BaselineReason = entry.Reason
			result.BaselineFailureOutput = entry.FailureOutput
			result.StatusDelta = computeStatusDelta(result.Status, result.BaselineStatus)
		}

		builds = append(builds, result)
	}

	sort.Slice(builds, func(i, j int) bool {
		priority := map[BuildStatus]int{
			BuildStatusFailed:    0,
			BuildStatusUnknown:   1,
			BuildStatusSucceeded: 2,
		}
		if priority[builds[i].Status] != priority[builds[j].Status] {
			return priority[builds[i].Status] < priority[builds[j].Status]
		}
		return builds[i].Name < builds[j].Name
	})

	return builds, nil
}

func parseLog(path string) (BuildResult, error) {
	var result BuildResult

	data, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("read file: %w", err)
	}
	content := string(data)

	result.RunCommand = findRunCommand(content)
	result.ErrorCommand = findErrorCommand(content)
	result.ErrorOutput = findErrorOutput(content)
	result.Status = determineStatus(content, result.ErrorCommand)

	return result, nil
}

type baselineSummary struct {
	Entries []baselineEntry `json:"entries"`
}

type baselineEntry struct {
	Name          string `json:"name"`
	Recipe        string `json:"recipe"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
	FailureOutput string `json:"failure_output"`
}

func loadBaseline(path string) (map[string]baselineEntry, bool, error) {
	baseline := make(map[string]baselineEntry)
	if path == "" {
		return baseline, false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return baseline, false, nil
		}
		return nil, false, fmt.Errorf("reading %q: %w", path, err)
	}

	var summary baselineSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, false, fmt.Errorf("decoding %q: %w", path, err)
	}

	for _, entry := range summary.Entries {
		key := normalizeRecipeName(entry.Recipe)
		if key == "" {
			key = deriveRecipeFromName(entry.Name)
		}
		if key == "" {
			continue
		}
		baseline[key] = entry
	}

	return baseline, true, nil
}

func deriveRecipeFromName(name string) string {
	if name == "" {
		return ""
	}
	normalized := strings.ToLower(name)
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.ReplaceAll(normalized, "-", " ")
	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return ""
	}
	for len(fields) > 0 {
		if _, err := strconv.Atoi(strings.Trim(fields[0], ":")); err == nil {
			fields = fields[1:]
			continue
		}
		break
	}
	if len(fields) == 0 {
		return ""
	}
	for len(fields) > 0 {
		switch fields[0] {
		case "recipe", "recipes", "build":
			fields = fields[1:]
			continue
		}
		break
	}
	if len(fields) == 0 {
		return ""
	}
	return normalizeRecipeName(fields[0])
}

func normalizeRecipeName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ""
	}
	n = strings.Trim(n, ".,:;")
	for _, cut := range []string{" ", "_", "-"} {
		if strings.Contains(n, cut) {
			n = strings.ReplaceAll(n, cut, "")
		}
	}
	for _, suffix := range []string{".sif", ".simg", ".img", ".squashfs", ".tar", ".tgz", ".gz"} {
		if strings.HasSuffix(n, suffix) {
			n = strings.TrimSuffix(n, suffix)
		}
	}
	return n
}

func normalizeBaselineStatus(status string) BuildStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded", "success":
		return BuildStatusSucceeded
	case "failed", "failure", "error":
		return BuildStatusFailed
	default:
		return BuildStatusUnknown
	}
}

func computeStatusDelta(current, baseline BuildStatus) string {
	if baseline == BuildStatusUnknown || current == BuildStatusUnknown {
		return ""
	}
	if current == baseline {
		return ""
	}
	if baseline == BuildStatusFailed && current == BuildStatusSucceeded {
		return "Improved"
	}
	if baseline == BuildStatusSucceeded && current == BuildStatusFailed {
		return "Regressed"
	}
	return "Changed"
}

func findRunCommand(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "Running:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Running:"))
		}
	}
	return ""
}

func findErrorCommand(content string) string {
	match := errorCommandPattern.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return sanitizeCommand(match[1])
}

func findErrorOutput(content string) string {
	errorIdx := strings.LastIndex(content, `ERROR: process "`)
	searchStart := 0
	if errorIdx != -1 {
		searchStart = errorIdx
	}

	subject := content[searchStart:]
	blockStart := strings.Index(subject, "\n > ")
	if blockStart == -1 {
		blockStart = strings.Index(subject, "\n> ")
	}
	if blockStart == -1 {
		return ""
	}
	blockStart += searchStart + 1

	blockEnd := strings.Index(content[blockStart:], "\n------")
	var block string
	if blockEnd == -1 {
		block = content[blockStart:]
	} else {
		block = content[blockStart : blockStart+blockEnd]
	}

	return strings.Trim(block, "\n")
}

func sanitizeCommand(raw string) string {
	if raw == "" {
		return ""
	}

	if unquoted, err := strconv.Unquote("\"" + raw + "\""); err == nil {
		return strings.TrimSpace(unquoted)
	}

	replacer := strings.NewReplacer(
		"\\n", "\n",
		"\\t", "\t",
		"\\\"", "\"",
		"\\\\", "\\",
	)

	return strings.TrimSpace(replacer.Replace(raw))
}

func determineStatus(content, errorCommand string) BuildStatus {
	failureMarkers := []string{
		"docker build failed",
		"ERROR: failed to build",
		"fatal error=",
	}

	for _, marker := range failureMarkers {
		if strings.Contains(content, marker) {
			return BuildStatusFailed
		}
	}
	if errorCommand != "" {
		return BuildStatusFailed
	}
	if strings.Contains(content, "Built image") {
		return BuildStatusSucceeded
	}
	return BuildStatusUnknown
}

func statusLabel(status BuildStatus) string {
	switch status {
	case BuildStatusSucceeded:
		return "Succeeded"
	case BuildStatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

func statusBadgeClass(status BuildStatus) string {
	switch status {
	case BuildStatusSucceeded:
		return "bg-emerald-500/10 text-emerald-300 border border-emerald-500/30"
	case BuildStatusFailed:
		return "bg-rose-500/10 text-rose-200 border border-rose-500/40"
	default:
		return "bg-amber-500/10 text-amber-200 border border-amber-500/40"
	}
}

func statusBorderClass(status BuildStatus) string {
	switch status {
	case BuildStatusSucceeded:
		return "border-emerald-500/40"
	case BuildStatusFailed:
		return "border-rose-500/50"
	default:
		return "border-slate-800"
	}
}

func statusDeltaClass(delta string) string {
	switch delta {
	case "Improved":
		return "bg-emerald-500/10 text-emerald-200 border border-emerald-500/30"
	case "Regressed":
		return "bg-rose-500/10 text-rose-200 border border-rose-500/30"
	case "Changed":
		return "bg-amber-500/10 text-amber-200 border border-amber-500/30"
	default:
		return "bg-slate-800 text-slate-300 border border-slate-700"
	}
}

const dashboardTemplateHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Docker Build Status Dashboard</title>
<script src="https://cdn.tailwindcss.com?plugins=forms,typography"></script>
</head>
<body class="bg-slate-950 text-slate-100">
  <div class="mx-auto max-w-7xl space-y-6 px-4 py-10">
    <header class="space-y-2">
      <h1 class="text-3xl font-semibold tracking-tight">Docker Build Status</h1>
      <p class="text-sm text-slate-400">Generated {{.GeneratedAt.Format "2006-01-02 15:04:05 MST"}} from logs in <span class="font-mono text-slate-200">{{.LogsDir}}</span></p>
      {{if .HasBaseline}}
      <p class="text-sm text-slate-400">Baseline comparison: <span class="font-mono text-slate-200">{{.BaselinePath}}</span></p>
      {{end}}
    </header>
    {{if not .Builds}}
    <p class="rounded-md border border-slate-800 bg-slate-900/80 px-4 py-6 text-slate-300">No build logs found.</p>
    {{else}}
    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      {{range .Builds}}
      <article class="rounded-lg border {{statusBorder .Status}} bg-slate-900/80 p-4 shadow-sm">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h2 class="text-lg font-semibold text-slate-100">{{.Name}}</h2>
            <p class="text-xs text-slate-400">Updated {{.LastModified.Format "2006-01-02 15:04 MST"}}</p>
          </div>
          <span class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium tracking-wide {{statusBadge .Status}}">{{statusLabel .Status}}</span>
        </div>
        <dl class="mt-4 space-y-3 text-sm text-slate-300">
          <div>
            <dt class="font-medium text-slate-200">Log file</dt>
            <dd class="font-mono text-xs text-slate-400">{{.LogRelative}}</dd>
          </div>
          {{if $.HasBaseline}}
          <div>
            <dt class="font-medium text-slate-200">Baseline</dt>
            <dd>
              {{if .BaselineProvided}}
              <div class="flex flex-wrap items-center gap-2">
                <span class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium tracking-wide {{statusBadge .BaselineStatus}}">{{statusLabel .BaselineStatus}}</span>
                {{if .StatusDelta}}
                <span class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide {{deltaClass .StatusDelta}}">{{.StatusDelta}}</span>
                {{end}}
              </div>
              {{if .BaselineReason}}
              <p class="mt-1 whitespace-pre-wrap text-xs text-slate-400">{{.BaselineReason}}</p>
              {{end}}
              {{if and .BaselineFailureOutput (eq .StatusDelta "Improved")}}
              <details class="mt-2">
                <summary class="cursor-pointer text-xs text-emerald-300 hover:text-emerald-200">View baseline failure output</summary>
                <pre class="mt-1 whitespace-pre-wrap rounded border border-emerald-700/40 bg-emerald-950/30 p-3 text-xs text-emerald-100 overflow-x-auto">{{.BaselineFailureOutput}}</pre>
              </details>
              {{end}}
              {{else}}
              <p class="text-xs italic text-slate-500">No baseline entry.</p>
              {{end}}
            </dd>
          </div>
          {{end}}
          {{if .RunCommand}}
          <div>
            <dt class="font-medium text-slate-200">Docker invocation</dt>
            <dd>
              <pre class="mt-1 whitespace-pre-wrap rounded border border-slate-800 bg-slate-950/60 p-3 text-xs text-slate-200 overflow-x-auto">{{.RunCommand}}</pre>
            </dd>
          </div>
          {{end}}
          {{if eq .Status "failed"}}
          {{if .ErrorCommand}}
          <div>
            <dt class="font-medium text-rose-300">Command line</dt>
            <dd>
              <pre class="mt-1 whitespace-pre-wrap rounded border border-rose-700/40 bg-rose-950/40 p-3 text-xs text-rose-100 overflow-x-auto">{{.ErrorCommand}}</pre>
            </dd>
          </div>
          {{end}}
          {{if .ErrorOutput}}
          <div>
            <dt class="font-medium text-rose-300">Command output</dt>
            <dd>
              <pre class="mt-1 whitespace-pre-wrap rounded border border-rose-700/40 bg-rose-950/30 p-3 text-xs text-rose-100 overflow-x-auto">{{.ErrorOutput}}</pre>
            </dd>
          </div>
          {{end}}
          {{end}}
        </dl>
      </article>
      {{end}}
    </div>
    {{end}}
  </div>
</body>
</html>`
