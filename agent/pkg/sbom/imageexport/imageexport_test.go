package imageexport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeExporter struct {
	data  []byte
	err   error
	calls []string
}

func (f *fakeExporter) Export(_ context.Context, ref string, w io.Writer) error {
	f.calls = append(f.calls, ref)
	if _, err := w.Write(f.data); err != nil {
		return err
	}
	return f.err
}

func startServer(t *testing.T, exp Exporter, maxBytes int64) string {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "export.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	srv := &Server{Exporter: exp, Timeout: 5 * time.Second, MaxBytes: maxBytes, Concurrency: 1}
	go func() { _ = srv.Serve(ctx, ln) }()
	return socket
}

func TestExportStreamsArchive(t *testing.T) {
	data := bytes.Repeat([]byte("layer"), 50_000) // spans several frames
	exp := &fakeExporter{data: data}
	socket := startServer(t, exp, 0)
	var out bytes.Buffer
	if err := Export(context.Background(), socket, "docker.io/library/nginx:1.27", &out, 0); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("archive mismatch: got %d bytes want %d", out.Len(), len(data))
	}
	if len(exp.calls) != 1 || exp.calls[0] != "index.docker.io/library/nginx:1.27" {
		t.Fatalf("exporter called with %v", exp.calls)
	}
}

func TestExportReportsHelperError(t *testing.T) {
	socket := startServer(t, &fakeExporter{data: []byte("partial"), err: errors.New("image not found")}, 0)
	err := Export(context.Background(), socket, "registry.example/app:v1", io.Discard, 0)
	if err == nil || !strings.Contains(err.Error(), "image not found") {
		t.Fatalf("expected helper error, got %v", err)
	}
}

func TestServerEnforcesArchiveLimit(t *testing.T) {
	socket := startServer(t, &fakeExporter{data: make([]byte, 200_000)}, 100_000)
	if err := Export(context.Background(), socket, "registry.example/app:v1", io.Discard, 0); err == nil {
		t.Fatal("expected size limit error")
	}
}

func TestClientEnforcesArchiveLimit(t *testing.T) {
	socket := startServer(t, &fakeExporter{data: make([]byte, 200_000)}, 0)
	if err := Export(context.Background(), socket, "registry.example/app:v1", io.Discard, 100_000); err == nil {
		t.Fatal("expected client size limit error")
	}
}

func TestServerRejectsHostileRequests(t *testing.T) {
	exp := &fakeExporter{data: []byte("x")}
	socket := startServer(t, exp, 0)
	for name, raw := range map[string]string{
		"not json":        "nginx\n",
		"unknown field":   `{"image":"registry.example/app:v1","path":"/etc/shadow"}` + "\n",
		"short reference": `{"image":"nginx"}` + "\n",
		"path traversal":  `{"image":"registry.example/../../etc:v1"}` + "\n",
		"no newline":      `{"image":"registry.example/app:v1"}`,
		"oversized":       `{"image":"` + strings.Repeat("a", 2000) + `"}` + "\n",
	} {
		conn, err := net.Dial("unix", socket)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = conn.Write([]byte(raw))
		if c, ok := conn.(*net.UnixConn); ok {
			_ = c.CloseWrite()
		}
		kind, _, err := readFrame(conn)
		conn.Close()
		if err != nil || kind != frameError {
			t.Fatalf("%s: expected error frame, got %q (%v)", name, kind, err)
		}
	}
	if len(exp.calls) != 0 {
		t.Fatalf("exporter must not run for rejected requests: %v", exp.calls)
	}
}

func TestValidateReference(t *testing.T) {
	if _, err := ValidateReference("registry.example/team/app@sha256:" + strings.Repeat("a", 64)); err != nil {
		t.Fatalf("digest reference rejected: %v", err)
	}
	for _, bad := range []string{"", "nginx", "nginx:latest", "registry.example/app", "registry.example/app:v1 --flag"} {
		if _, err := ValidateReference(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
