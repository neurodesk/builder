package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type testResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type agent struct {
	visited map[string]struct{}
	tests   []testResult

	total   int
	passed  int
	failed  int
	skipped int
}

func newAgent() *agent {
	return &agent{
		visited: make(map[string]struct{}),
	}
}

func (a *agent) recordResult(name, status, message string) {
	a.tests = append(a.tests, testResult{Name: name, Status: status, Message: message})
	a.total++
	switch status {
	case "passed":
		a.passed++
	case "failed":
		a.failed++
	case "skipped":
		a.skipped++
	}
}

func (a *agent) hasVisited(path string) bool {
	if _, ok := a.visited[path]; ok {
		return true
	}
	a.visited[path] = struct{}{}
	return false
}

func (a *agent) processDeployBins() {
	bins := os.Getenv("DEPLOY_BINS")
	if strings.TrimSpace(bins) == "" {
		a.recordResult("deploy_bins", "skipped", "DEPLOY_BINS not set.")
		return
	}

	for _, entry := range strings.Split(bins, ":") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		resolved := resolveBinary(entry)
		if resolved != "" {
			a.recordResult(fmt.Sprintf("deploy_bin:%s", entry), "passed", fmt.Sprintf("Binary %s found at %s.", entry, resolved))
			a.testFile(resolved)
			continue
		}
		a.recordResult(fmt.Sprintf("deploy_bin:%s", entry), "failed", fmt.Sprintf("Binary %s not found on PATH.", entry))
	}
}

func (a *agent) processDeployPaths() {
	paths := os.Getenv("DEPLOY_PATH")
	if strings.TrimSpace(paths) == "" {
		a.recordResult("deploy_path", "skipped", "DEPLOY_PATH not set.")
		return
	}

	for _, dir := range strings.Split(paths, ":") {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}

		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			a.recordResult(fmt.Sprintf("deploy_dir:%s", dir), "failed", fmt.Sprintf("Directory %s does not exist.", dir))
			continue
		}

		a.recordResult(fmt.Sprintf("deploy_dir:%s", dir), "passed", fmt.Sprintf("Directory %s exists.", dir))

		entries, err := os.ReadDir(dir)
		if err != nil {
			a.recordResult(fmt.Sprintf("deploy_dir.read:%s", dir), "failed", fmt.Sprintf("Unable to read directory %s: %v", dir, err))
			continue
		}

		for _, entry := range entries {
			fullPath := filepath.Join(dir, entry.Name())

			stat, err := os.Stat(fullPath)
			if err != nil {
				a.testFile(fullPath)
				continue
			}

			if stat.IsDir() {
				continue
			}

			if stat.Mode()&0o111 != 0o111 {
				continue
			}

			a.testFile(fullPath)
		}
	}
}

func (a *agent) testFile(filename string) {
	if filename == "" {
		return
	}

	if a.hasVisited(filename) {
		return
	}

	info, err := os.Stat(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			a.recordResult(fmt.Sprintf("file.exists:%s", filename), "failed", fmt.Sprintf("File %s does not exist.", filename))
		} else {
			a.recordResult(fmt.Sprintf("file.exists:%s", filename), "failed", fmt.Sprintf("Unable to stat %s: %v", filename, err))
		}
		return
	}

	if info.IsDir() {
		a.recordResult(fmt.Sprintf("file.directory:%s", filename), "skipped", fmt.Sprintf("Path %s is a directory.", filename))
		return
	}

	a.recordResult(fmt.Sprintf("file.exists:%s", filename), "passed", fmt.Sprintf("File %s exists.", filename))

	if !isExecutable(filename) {
		a.recordResult(fmt.Sprintf("file.executable:%s", filename), "failed", fmt.Sprintf("File %s is not executable.", filename))
		return
	}

	a.recordResult(fmt.Sprintf("file.executable:%s", filename), "passed", fmt.Sprintf("File %s is executable.", filename))

	a.testFileLinking(filename)
}

func (a *agent) testFileLinking(filename string) {
	isELF, interpreter, err := detectELF(filename)
	if err != nil {
		a.recordResult(fmt.Sprintf("file.type:%s", filename), "failed", fmt.Sprintf("Failed to inspect %s: %v", filename, err))
		return
	}

	if isELF {
		if interpreter != "" {
			a.recordResult(fmt.Sprintf("elf.interpreter:%s", filename), "passed", fmt.Sprintf("Interpreter %s", interpreter))
			a.testFile(interpreter)
		} else {
			a.recordResult(fmt.Sprintf("elf.interpreter:%s", filename), "skipped", "No PT_INTERP segment present.")
		}

		output, err := exec.Command("ldd", filename).CombinedOutput()
		outStr := string(output)
		if err == nil {
			a.recordResult(fmt.Sprintf("file.linkage:%s", filename), "passed", fmt.Sprintf("File %s is dynamically linked.", filename))
			a.parseLddOutput(filename, outStr)
			return
		}

		if strings.Contains(outStr, "not a dynamic executable") || strings.Contains(outStr, "statically linked") {
			a.recordResult(fmt.Sprintf("file.linkage:%s", filename), "passed", fmt.Sprintf("File %s is statically linked.", filename))
			return
		}

		a.recordResult(fmt.Sprintf("file.linkage:%s", filename), "failed", fmt.Sprintf("ldd error: %s", strings.TrimSpace(outStr)))
		return
	}

	firstLine, err := readFirstLine(filename)
	if err != nil {
		if errors.Is(err, io.EOF) {
			a.recordResult(fmt.Sprintf("file.read:%s", filename), "failed", fmt.Sprintf("Unable to read file header for %s.", filename))
		} else {
			a.recordResult(fmt.Sprintf("file.read:%s", filename), "failed", fmt.Sprintf("Unable to read file header for %s: %v", filename, err))
		}
		return
	}

	if strings.HasPrefix(firstLine, "#!") {
		interpreterLine := strings.TrimSpace(firstLine[2:])
		if interpreterLine == "" {
			a.recordResult(fmt.Sprintf("script:%s", filename), "failed", fmt.Sprintf("Script %s has empty interpreter directive.", filename))
			return
		}

		interpreter, args := splitInterpreter(interpreterLine)
		msg := fmt.Sprintf("Script uses interpreter: %s", interpreter)
		if args != "" {
			msg += " " + args
		}
		a.recordResult(fmt.Sprintf("script:%s", filename), "passed", msg)

		resolved := normalisePath(interpreter)
		if resolved != "" {
			a.testFile(resolved)
		} else {
			a.recordResult(fmt.Sprintf("script.interpreter:%s", filename), "failed", fmt.Sprintf("Interpreter %s not found on PATH.", interpreter))
		}
		return
	}

	a.recordResult(fmt.Sprintf("file.type:%s", filename), "skipped", fmt.Sprintf("File %s is not an ELF binary or recognised script.", filename))
}

func (a *agent) parseLddOutput(binary, output string) {
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		label := fields[0]
		path := ""

		if strings.Contains(line, "=>") {
			for _, field := range fields {
				if strings.HasPrefix(field, "/") {
					path = field
					break
				}
			}
		} else if strings.HasPrefix(line, "/") {
			path = fields[0]
		}

		if path != "" && path != "not" {
			if _, err := os.Stat(path); err == nil {
				a.recordResult(fmt.Sprintf("ldd:%s:%s", binary, label), "passed", fmt.Sprintf("Library %s resolved to %s.", label, path))
				continue
			}
			a.recordResult(fmt.Sprintf("ldd:%s:%s", binary, label), "failed", fmt.Sprintf("Library %s missing (expected at %s).", label, path))
			continue
		}
		a.recordResult(fmt.Sprintf("ldd:%s:%s", binary, label), "skipped", fmt.Sprintf("No filesystem path to validate for %s.", label))
	}
}

func readFirstLine(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	line, err := reader.ReadString('\n')
	if errors.Is(err, io.EOF) {
		if len(line) == 0 {
			return "", io.EOF
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func splitInterpreter(line string) (string, string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", ""
	}
	interpreter := fields[0]
	if len(fields) == 1 {
		return interpreter, ""
	}
	return interpreter, strings.Join(fields[1:], " ")
}

func normalisePath(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return value
	}
	resolved, err := exec.LookPath(value)
	if err != nil {
		return ""
	}
	return resolved
}

func resolveBinary(entry string) string {
	if entry == "" {
		return ""
	}

	if filepath.IsAbs(entry) || strings.HasPrefix(entry, ".") {
		if stat, err := os.Stat(entry); err == nil && !stat.IsDir() {
			return entry
		}
	}

	resolved, err := exec.LookPath(entry)
	if err != nil {
		return ""
	}
	return resolved
}

func isExecutable(path string) bool {
	if err := unix.Access(path, unix.X_OK); err == nil {
		return true
	}

	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.Mode()&0o111 == 0o111
}

func (a *agent) report() ([]byte, error) {
	payload := map[string]any{
		"total":   a.total,
		"passed":  a.passed,
		"failed":  a.failed,
		"skipped": a.skipped,
		"tests":   a.tests,
	}
	return json.MarshalIndent(payload, "", "  ")
}

func main() {
	log.SetFlags(0)

	ag := newAgent()
	ag.processDeployBins()
	ag.processDeployPaths()

	report, err := ag.report()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to produce report: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(report))

	if ag.failed > 0 {
		os.Exit(1)
	}
}

const (
	elfClass32 = 1
	elfClass64 = 2
	elfDataLSB = 1
	elfDataMSB = 2
	ptInterp   = 3
)

type elfHeader32 struct {
	Type      uint16
	Machine   uint16
	Version   uint32
	Entry     uint32
	Phoff     uint32
	Shoff     uint32
	Flags     uint32
	Ehsize    uint16
	Phentsize uint16
	Phnum     uint16
	Shentsize uint16
	Shnum     uint16
	Shstrndx  uint16
}

type elfHeader64 struct {
	Type      uint16
	Machine   uint16
	Version   uint32
	Entry     uint64
	Phoff     uint64
	Shoff     uint64
	Flags     uint32
	Ehsize    uint16
	Phentsize uint16
	Phnum     uint16
	Shentsize uint16
	Shnum     uint16
	Shstrndx  uint16
}

type programHeader32 struct {
	Type   uint32
	Offset uint32
	Vaddr  uint32
	Paddr  uint32
	Filesz uint32
	Memsz  uint32
	Flags  uint32
	Align  uint32
}

type programHeader64 struct {
	Type   uint32
	Flags  uint32
	Offset uint64
	Vaddr  uint64
	Paddr  uint64
	Filesz uint64
	Memsz  uint64
	Align  uint64
}

func detectELF(path string) (bool, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, "", err
	}
	defer f.Close()

	var ident [16]byte
	if _, err := io.ReadFull(f, ident[:]); err != nil {
		return false, "", err
	}

	if ident[0] != 0x7f || ident[1] != 'E' || ident[2] != 'L' || ident[3] != 'F' {
		return false, "", nil
	}

	var order binary.ByteOrder
	switch ident[5] {
	case elfDataLSB:
		order = binary.LittleEndian
	case elfDataMSB:
		order = binary.BigEndian
	default:
		return true, "", fmt.Errorf("unsupported ELF data encoding %d", ident[5])
	}

	switch ident[4] {
	case elfClass32:
		var header elfHeader32
		if err := binary.Read(f, order, &header); err != nil {
			return true, "", fmt.Errorf("reading ELF32 header: %w", err)
		}
		interp, err := readELFInterpreter32(f, order, header)
		return true, interp, err
	case elfClass64:
		var header elfHeader64
		if err := binary.Read(f, order, &header); err != nil {
			return true, "", fmt.Errorf("reading ELF64 header: %w", err)
		}
		interp, err := readELFInterpreter64(f, order, header)
		return true, interp, err
	default:
		return true, "", fmt.Errorf("unsupported ELF class %d", ident[4])
	}
}

func readELFInterpreter32(f *os.File, order binary.ByteOrder, header elfHeader32) (string, error) {
	if header.Phoff == 0 || header.Phnum == 0 {
		return "", nil
	}
	entrySize := int(header.Phentsize)
	if entrySize <= 0 {
		return "", nil
	}
	reader := io.NewSectionReader(f, int64(header.Phoff), int64(entrySize)*int64(header.Phnum))
	structSize := binary.Size(programHeader32{})
	for i := 0; i < int(header.Phnum); i++ {
		var ph programHeader32
		if err := binary.Read(reader, order, &ph); err != nil {
			return "", fmt.Errorf("reading ELF32 program header: %w", err)
		}
		if extra := int64(entrySize - structSize); extra > 0 {
			if _, err := reader.Seek(extra, io.SeekCurrent); err != nil {
				return "", err
			}
		}
		if ph.Type != ptInterp {
			continue
		}
		sz := int64(ph.Filesz)
		if sz <= 0 {
			return "", nil
		}
		if sz > 1<<16 {
			return "", fmt.Errorf("unexpected ELF interpreter size %d", sz)
		}
		buf := make([]byte, sz)
		if _, err := f.ReadAt(buf, int64(ph.Offset)); err != nil {
			return "", fmt.Errorf("reading ELF interpreter: %w", err)
		}
		return strings.TrimRight(string(buf), "\x00"), nil
	}
	return "", nil
}

func readELFInterpreter64(f *os.File, order binary.ByteOrder, header elfHeader64) (string, error) {
	if header.Phoff == 0 || header.Phnum == 0 {
		return "", nil
	}
	entrySize := int(header.Phentsize)
	if entrySize <= 0 {
		return "", nil
	}
	reader := io.NewSectionReader(f, int64(header.Phoff), int64(entrySize)*int64(header.Phnum))
	structSize := binary.Size(programHeader64{})
	for i := 0; i < int(header.Phnum); i++ {
		var ph programHeader64
		if err := binary.Read(reader, order, &ph); err != nil {
			return "", fmt.Errorf("reading ELF64 program header: %w", err)
		}
		if extra := int64(entrySize - structSize); extra > 0 {
			if _, err := reader.Seek(extra, io.SeekCurrent); err != nil {
				return "", err
			}
		}
		if ph.Type != ptInterp {
			continue
		}
		sz := int64(ph.Filesz)
		if sz <= 0 {
			return "", nil
		}
		if sz > 1<<16 {
			return "", fmt.Errorf("unexpected ELF interpreter size %d", sz)
		}
		buf := make([]byte, sz)
		if _, err := f.ReadAt(buf, int64(ph.Offset)); err != nil {
			return "", fmt.Errorf("reading ELF interpreter: %w", err)
		}
		return strings.TrimRight(string(buf), "\x00"), nil
	}
	return "", nil
}
