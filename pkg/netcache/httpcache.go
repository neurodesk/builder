package netcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cache provides a simple persistent HTTP cache with ETag/Last-Modified support.
type Cache struct {
	Dir    string
	Client *http.Client
}

// New returns a new Cache with a reasonable default HTTP client.
func New(dir string) *Cache {
	return &Cache{
		Dir: dir,
		Client: &http.Client{
			Timeout: 30 * time.Minute, // long timeout for large artifacts
		},
	}
}

type meta struct {
	URL          string `json:"url"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
	// Optional server-provided filename hint
	Filename string `json:"filename,omitempty"`
	// DataFile is the basename of the cached payload file
	DataFile string `json:"data_file"`
}

// Get fetches the URL into the cache and returns a local file path.
// If the cache is valid, it is reused without downloading.
// Returns (path, fromCache, error).
func (c *Cache) Get(ctx context.Context, url string) (string, bool, error) {
	key := hash(url)
	mpath := filepath.Join(c.Dir, key+".json")
	var m meta
	var haveMeta bool
	if b, err := os.ReadFile(mpath); err == nil {
		_ = json.Unmarshal(b, &m)
		// Validate basic consistency
		if m.URL == url && m.DataFile != "" {
			if _, err := os.Stat(filepath.Join(c.Dir, m.DataFile)); err == nil {
				haveMeta = true
			}
		}
	}

	// If we have metadata, try a conditional GET
	if haveMeta {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false, err
		}
		if m.ETag != "" {
			req.Header.Set("If-None-Match", m.ETag)
		}
		if m.LastModified != "" {
			req.Header.Set("If-Modified-Since", m.LastModified)
		}
		resp, err := c.Client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusNotModified {
				return filepath.Join(c.Dir, m.DataFile), true, nil
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				// Update cache with new body
				dataFile := key + ".data"
				path := filepath.Join(c.Dir, dataFile)
				if verboseEnabled() {
					if err := streamToFileWithProgress(resp.Body, path, 0o644, resp.ContentLength, contentFilename(url, resp)); err != nil {
						return "", false, err
					}
				} else {
					if err := streamToFile(resp.Body, path, 0o644); err != nil {
						return "", false, err
					}
				}
				nm := meta{
					URL:          url,
					ETag:         resp.Header.Get("ETag"),
					LastModified: resp.Header.Get("Last-Modified"),
					Filename:     contentFilename(url, resp),
					DataFile:     dataFile,
				}
				if err := writeMeta(mpath, nm); err != nil {
					return "", false, err
				}
				return path, false, nil
			}
			// Fall through on non-success codes
		}
		// If conditional request fails (network or server), reuse cached file best-effort
		if p := filepath.Join(c.Dir, m.DataFile); fileExists(p) {
			return p, true, nil
		}
		// Else continue to full fetch below
	}

	// Full fetch with simple retry/backoff on network errors or 5xx
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false, err
		}
		resp, err := c.Client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			func() {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					dataFile := key + ".data"
					path := filepath.Join(c.Dir, dataFile)
					var err error
					if verboseEnabled() {
						err = streamToFileWithProgress(resp.Body, path, 0o644, resp.ContentLength, contentFilename(url, resp))
					} else {
						err = streamToFile(resp.Body, path, 0o644)
					}
					if err != nil {
						lastErr = err
						return
					}
					nm := meta{
						URL:          url,
						ETag:         resp.Header.Get("ETag"),
						LastModified: resp.Header.Get("Last-Modified"),
						Filename:     contentFilename(url, resp),
						DataFile:     dataFile,
					}
					if err := writeMeta(mpath, nm); err != nil {
						lastErr = err
						return
					}
					lastErr = nil
					// success
				} else {
					lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				}
			}()
		}
		if lastErr == nil {
			// Success; return cached path derived from meta
			dataFile := key + ".data"
			return filepath.Join(c.Dir, dataFile), false, nil
		}
		// Backoff before retrying
		time.Sleep(time.Duration(1<<attempt) * 2 * time.Second)
	}
	return "", false, lastErr
}

func streamToFile(r io.Reader, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// streamToFileWithProgress writes r to dst while printing a status line with
// downloaded bytes, speed, and ETA (when total >= 0). Progress is printed to stderr.
func streamToFileWithProgress(r io.Reader, dst string, mode os.FileMode, total int64, label string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	pr := &progressReporter{total: total, label: label, start: time.Now(), lastTick: time.Now()}
	reader := io.TeeReader(r, pr)
	_, copyErr := io.Copy(f, reader)
	closeErr := f.Close()
	pr.finish(copyErr == nil && closeErr == nil)
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dst)
}

type progressReporter struct {
	total    int64
	read     int64
	label    string
	start    time.Time
	lastTick time.Time
}

func humanBytes(n int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(GB))
	case n >= MB:
		return fmt.Sprintf("%.2f MB", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func (p *progressReporter) Write(b []byte) (int, error) {
	n := len(b)
	p.read += int64(n)
	now := time.Now()
	if now.Sub(p.lastTick) >= 200*time.Millisecond {
		p.print(now)
		p.lastTick = now
	}
	return n, nil
}

func (p *progressReporter) finish(ok bool) {
	// Final print and newline
	p.print(time.Now())
	if ok {
		fmt.Fprint(os.Stderr, "\n")
	} else {
		fmt.Fprint(os.Stderr, "\n")
	}
}

func (p *progressReporter) print(now time.Time) {
	elapsed := now.Sub(p.start).Seconds()
	if elapsed <= 0 {
		elapsed = 0.001
	}
	speed := float64(p.read) / elapsed // bytes/sec
	var etaStr string
	if p.total > 0 && speed > 0 {
		remain := float64(p.total - p.read)
		if remain < 0 {
			remain = 0
		}
		eta := time.Duration(remain/speed) * time.Second
		etaStr = eta.String()
	} else {
		etaStr = "?-?"
	}
	var totalStr string
	if p.total > 0 {
		totalStr = humanBytes(p.total)
	} else {
		totalStr = "unknown"
	}
	line := fmt.Sprintf("\rDownloading %s: %s / %s at %.2f MB/s, ETA %s",
		p.label,
		humanBytes(p.read),
		totalStr,
		speed/1024.0/1024.0,
		etaStr,
	)
	// Avoid overly long lines
	if len(line) > 120 {
		line = line[:120]
	}
	fmt.Fprint(os.Stderr, line)
}

func writeMeta(path string, m meta) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// contentFilename tries to derive a filename from headers or URL path.
func contentFilename(url string, resp *http.Response) string {
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		// very light parse; look for filename="..."
		if i := strings.Index(cd, "filename="); i >= 0 {
			v := cd[i+9:]
			v = strings.Trim(v, "\"'")
			if v != "" {
				return v
			}
		}
	}
	// Fallback to last path segment
	slash := strings.LastIndex(url, "/")
	if slash >= 0 && slash+1 < len(url) {
		return url[slash+1:]
	}
	return "download"
}

var _ = errors.New // keep errors imported if not used in some builds

// verboseEnabled reports whether verbose output is enabled.
// It is controlled by the environment variable BUILDER_VERBOSE.
func verboseEnabled() bool {
	v := os.Getenv("BUILDER_VERBOSE")
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
