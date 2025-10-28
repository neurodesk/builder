package ir

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	bkclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
)

type EventType string

const (
	EventTypeStatus EventType = "status"
	EventTypeError  EventType = "error"
	EventTypeResult EventType = "result"
)

// Event carries raw BuildKit status plus a per-vertex name index derived
// from the LLB metadata (llb.WithCustomName). The key is the vertex digest
// string (e.g., "sha256:..."), the value is the original/custom name.
type Event struct {
	Type        EventType               `json:"type"`
	Status      *bkclient.SolveStatus   `json:"status,omitempty"`
	Error       string                  `json:"error,omitempty"`
	Result      *bkclient.SolveResponse `json:"result,omitempty"`
	VertexNames map[string]string       `json:"vertexNames,omitempty"`
}

// SubmitToDockerViaBuildx connects to the active buildx builder using
// "docker buildx dial-stdio", submits the LLB definition produced by
// your generator, and streams BuildKit status events as JSON.
// To surface original directive/source names in the stream, ensure your LLB
// generator sets llb.WithCustomName/WithCustomNamef per op; those names are
// extracted from llbDef.Metadata and exposed via Event.VertexNames.
func SubmitToDockerViaBuildx(
	ctx context.Context,
	llbDef *llb.Definition,
	builderName string, // empty means default builder
	localContextDir string, // e.g., "."
	outputChannel chan Event, // optional; if nil, falls back to stdout
) error {
	if llbDef == nil {
		return fmt.Errorf("empty LLB definition")
	}
	if outputChannel != nil {
		defer close(outputChannel)
	}

	// Derive an initial digest->name index from LLB metadata. This relies on
	// your generator using llb.WithCustomName to carry the "original names".
	vertexNames, err := buildVertexNameIndex(llbDef)
	if err != nil {
		return fmt.Errorf("building vertex name index: %w", err)
	}

	// Prepare a gRPC dialer that talks to buildx over stdio.
	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		return dialBuildxStdio(ctx, builderName)
	}

	// Connect BuildKit client via stdio.
	c, err := bkclient.New(
		ctx,
		"", // addr unused because we override with custom dialer
		bkclient.WithContextDialer(dialer),
	)
	if err != nil {
		return fmt.Errorf("buildkit client: %w", err)
	}
	defer c.Close()

	// Map local dirs for llb.Local("context") if used in the LLB.
	localDirs := map[string]string{}
	if localContextDir != "" {
		abs, err := filepath.Abs(localContextDir)
		if err != nil {
			return fmt.Errorf("resolve local context dir: %w", err)
		}
		localDirs["context"] = abs
	}

	statusCh := make(chan *bkclient.SolveStatus, 16)

	// Stream progress in a separate goroutine.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Single JSON encoder for stdout fallback.
		var enc *json.Encoder
		if outputChannel == nil {
			enc = json.NewEncoder(os.Stdout)
		}

		for s := range statusCh {
			// Enrich/patch vertex names when BuildKit supplies them; this also
			// captures names for vertices we didn't explicitly name.
			for i := range s.Vertexes {
				d := s.Vertexes[i].Digest.String()
				if s.Vertexes[i].Name != "" {
					vertexNames[d] = s.Vertexes[i].Name
				} else if n, ok := vertexNames[d]; ok && n != "" {
					// Populate missing Name for easier consumption.
					s.Vertexes[i].Name = n
				}
			}

			event := Event{
				Type:        EventTypeStatus,
				Status:      s,
				VertexNames: vertexNames,
			}

			if outputChannel != nil {
				outputChannel <- event
			} else {
				_ = enc.Encode(event)
			}
		}
	}()

	// Kick off the solve.
	resp, err := c.Solve(ctx, llbDef, bkclient.SolveOpt{
		LocalDirs: localDirs,
		// Add exporters here if you want to push or load the result, e.g.:
		// Exports: []bkclient.ExportEntry{
		// 	{
		// 		Type: bkclient.ExporterDocker, // or bkclient.ExporterOCI
		// 		Attrs: map[string]string{
		// 			"name": "your-image:tag",
		// 			"push": "true",
		// 		},
		// 	},
		// },
	}, statusCh)
	close(statusCh)
	wg.Wait()

	if err != nil {
		event := Event{
			Type:        EventTypeError,
			Error:       err.Error(),
			VertexNames: vertexNames,
		}
		if outputChannel != nil {
			outputChannel <- event
		} else {
			enc := json.NewEncoder(os.Stdout)
			_ = enc.Encode(event)
		}
		return err
	}

	// Emit final result line as JSON.
	event := Event{
		Type:        EventTypeResult,
		Result:      resp,
		VertexNames: vertexNames,
	}
	if outputChannel != nil {
		outputChannel <- event
	} else {
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(event)
	}

	return nil
}

// buildVertexNameIndex extracts digest->custom name mapping from LLB metadata.
// Names come from llb.WithCustomName/WithCustomNamef set during LLB creation.
func buildVertexNameIndex(def *llb.Definition) (map[string]string, error) {
	out := make(map[string]string, len(def.Metadata))
	_ = out
	for dgst, meta := range def.Metadata {
		customName, ok := meta.Description["lib.customname"]
		if !ok {
			continue
		}
		dgstStr := dgst.String()
		out[dgstStr] = customName
	}
	return out, nil
}

// dialBuildxStdio starts "docker buildx dial-stdio" and turns its stdio into a
// net.Conn suitable for gRPC. If builderName is empty, the default builder is
// used.
func dialBuildxStdio(ctx context.Context, builderName string) (net.Conn, error) {
	args := []string{"buildx", "dial-stdio"}
	if builderName != "" {
		args = append(args, "--builder", builderName)
	}
	cmd := exec.CommandContext(ctx, "docker", args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("dial-stdio stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("dial-stdio stdout: %w", err)
	}
	// Forward stderr for diagnostics.
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("dial-stdio start: %w", err)
	}

	return &stdioConn{
		r:   stdout,
		w:   stdin,
		cmd: cmd,
	}, nil
}

type stdioConn struct {
	r   io.ReadCloser
	w   io.WriteCloser
	cmd *exec.Cmd

	closeOnce sync.Once
}

func (c *stdioConn) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c *stdioConn) Write(p []byte) (int, error) { return c.w.Write(p) }
func (c *stdioConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		_ = c.w.Close()
		_ = c.r.Close()
		// Wait for the process to exit.
		err = c.cmd.Wait()
	})
	return err
}

func (c *stdioConn) LocalAddr() net.Addr                { return stdioAddr("local") }
func (c *stdioConn) RemoteAddr() net.Addr               { return stdioAddr("remote") }
func (c *stdioConn) SetDeadline(t time.Time) error      { return nil }
func (c *stdioConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *stdioConn) SetWriteDeadline(t time.Time) error { return nil }

type stdioAddr string

func (a stdioAddr) Network() string { return "stdio" }
func (a stdioAddr) String() string  { return string(a) }
