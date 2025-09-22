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
				if err := streamToFile(resp.Body, path, 0o644); err != nil {
					return "", false, err
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

	// Full fetch
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	dataFile := key + ".data"
	path := filepath.Join(c.Dir, dataFile)
	if err := streamToFile(resp.Body, path, 0o644); err != nil {
		return "", false, err
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
