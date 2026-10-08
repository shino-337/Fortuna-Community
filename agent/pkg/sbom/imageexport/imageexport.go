// Package imageexport moves the containerd socket out of the Agent container.
//
// The containerd socket gives full control of the node, while SBOM extraction
// parses untrusted image content. The image-export helper is the only container
// that mounts the socket; it has no Kubernetes or Core credentials and answers a
// single kind of request over a pod-local unix socket: "stream the image archive
// for this reference". The Agent parses the archive without socket access.
package imageexport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/platforms"
	"github.com/google/go-containerregistry/pkg/name"
)

const (
	// DefaultSocket is where the helper listens, on an emptyDir shared only
	// within the Agent pod.
	DefaultSocket = "/run/fortuna-image-export/export.sock"

	maxRequestBytes  = 1024
	maxReferenceLen  = 512
	maxErrorBytes    = 1024
	maxFrameBytes    = 64 << 10
	frameData        = 'D'
	frameError       = 'E'
	frameEnd         = 'K'
	frameHeaderBytes = 5
)

type request struct {
	Image string `json:"image"`
}

// ValidateReference accepts only a fully qualified image reference (registry,
// repository and tag or digest) and returns its canonical form.
func ValidateReference(s string) (string, error) {
	if s == "" || len(s) > maxReferenceLen || strings.ContainsAny(s, " \t\r\n") {
		return "", errors.New("invalid image reference")
	}
	ref, err := name.ParseReference(s, name.StrictValidation)
	if err != nil {
		return "", fmt.Errorf("invalid image reference: %w", err)
	}
	for _, part := range strings.Split(ref.Context().RepositoryStr(), "/") {
		if part == "" || strings.Trim(part, ".") == "" {
			return "", errors.New("invalid image reference: empty or dot path component")
		}
	}
	return ref.Name(), nil
}

// Exporter writes the image archive for a validated reference to w.
type Exporter interface {
	Export(ctx context.Context, ref string, w io.Writer) error
}

// ContainerdExporter exports images from the node's containerd content store.
type ContainerdExporter struct {
	Socket    string
	Namespace string
}

// Export writes an OCI archive of ref, trying docker.io/index.docker.io aliases.
func (c ContainerdExporter) Export(ctx context.Context, ref string, w io.Writer) error {
	client, err := containerd.New(c.Socket)
	if err != nil {
		return fmt.Errorf("containerd client error: %w", err)
	}
	defer client.Close()
	cctx := namespaces.WithNamespace(ctx, c.Namespace)
	for _, candidate := range ImageNames(ref) {
		if _, err := client.ImageService().Get(cctx, candidate); err != nil {
			continue
		}
		// Export only this node's platform. Without it containerd walks every manifest of a
		// multi-arch index and fails on the platforms the node never pulled ("content digest
		// ... not found"), and the archive has no manifest.json for the Agent to read.
		if err := archive.Export(cctx, client.ContentStore(), w,
			archive.WithImage(client.ImageService(), candidate),
			archive.WithPlatform(platforms.DefaultStrict())); err != nil {
			return fmt.Errorf("containerd export error: %w", err)
		}
		return nil
	}
	return fmt.Errorf("image not found in containerd: %s", ref)
}

// ImageNames returns the names containerd may store ref under.
func ImageNames(refName string) []string {
	names := []string{refName}
	if strings.HasPrefix(refName, "index.docker.io/") {
		names = append(names, strings.Replace(refName, "index.docker.io", "docker.io", 1))
	}
	if strings.HasPrefix(refName, "docker.io/") {
		names = append(names, strings.Replace(refName, "docker.io", "index.docker.io", 1))
	}
	return names
}

// Server answers export requests on a unix socket.
type Server struct {
	Exporter    Exporter
	Timeout     time.Duration // per request
	MaxBytes    int64         // per archive
	Concurrency int
	Logger      *log.Logger
}

// Serve accepts connections until ctx is done or the listener fails.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	slots := make(chan struct{}, max(1, s.Concurrency))
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case slots <- struct{}{}:
			go func() {
				defer func() { <-slots }()
				s.handle(ctx, conn)
			}()
		default:
			_ = writeFrame(conn, frameError, []byte("busy"))
			_ = conn.Close()
		}
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	_ = conn.SetDeadline(time.Now().Add(s.Timeout))
	start := time.Now()

	ref, err := readRequest(conn)
	if err != nil {
		s.logf("rejected request: %v", err)
		_ = writeFrame(conn, frameError, []byte(err.Error()))
		return
	}
	fw := &frameWriter{w: conn, limit: s.MaxBytes}
	if err := s.Exporter.Export(ctx, ref, fw); err != nil {
		s.logf("export %s failed after %d bytes: %v", ref, fw.n, err)
		_ = writeFrame(conn, frameError, []byte(err.Error()))
		return
	}
	_ = writeFrame(conn, frameEnd, nil)
	s.logf("exported %s (%d bytes, %s)", ref, fw.n, time.Since(start).Round(time.Millisecond))
}

func (s *Server) logf(format string, args ...any) {
	if s.Logger != nil {
		s.Logger.Printf(format, args...)
	}
}

func readRequest(r io.Reader) (string, error) {
	line, err := bufio.NewReaderSize(io.LimitReader(r, maxRequestBytes), maxRequestBytes).ReadBytes('\n')
	if err != nil {
		return "", errors.New("request must be one JSON line of at most 1 KiB")
	}
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	var req request
	if err := dec.Decode(&req); err != nil {
		return "", errors.New("malformed request")
	}
	return ValidateReference(req.Image)
}

// Export asks the helper at socket to stream the archive for ref into w.
func Export(ctx context.Context, socket, ref string, w io.Writer, maxBytes int64) error {
	canonical, err := ValidateReference(ref)
	if err != nil {
		return err
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return fmt.Errorf("image-export helper unavailable: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	defer stop()

	body, _ := json.Marshal(request{Image: canonical})
	if _, err := conn.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("send export request: %w", err)
	}
	br := bufio.NewReader(conn)
	var total int64
	for {
		kind, payload, err := readFrame(br)
		if err != nil {
			return fmt.Errorf("image-export response: %w", err)
		}
		switch kind {
		case frameData:
			total += int64(len(payload))
			if maxBytes > 0 && total > maxBytes {
				return fmt.Errorf("image archive exceeds %d bytes", maxBytes)
			}
			if _, err := w.Write(payload); err != nil {
				return err
			}
		case frameError:
			return fmt.Errorf("image-export helper: %s", payload)
		case frameEnd:
			return nil
		default:
			return fmt.Errorf("image-export response: unknown frame %q", kind)
		}
	}
}

type frameWriter struct {
	w     io.Writer
	n     int64
	limit int64
}

func (f *frameWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		chunk := p[:min(len(p), maxFrameBytes)]
		if f.limit > 0 && f.n+int64(len(chunk)) > f.limit {
			return written, fmt.Errorf("image archive exceeds %d bytes", f.limit)
		}
		if err := writeFrame(f.w, frameData, chunk); err != nil {
			return written, err
		}
		f.n += int64(len(chunk))
		written += len(chunk)
		p = p[len(chunk):]
	}
	return written, nil
}

func writeFrame(w io.Writer, kind byte, payload []byte) error {
	if kind == frameError && len(payload) > maxErrorBytes {
		payload = payload[:maxErrorBytes]
	}
	var header [frameHeaderBytes]byte
	header[0] = kind
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readFrame(r io.Reader) (byte, []byte, error) {
	var header [frameHeaderBytes]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	size := binary.BigEndian.Uint32(header[1:])
	limit := uint32(maxFrameBytes)
	if header[0] == frameError {
		limit = maxErrorBytes
	}
	if size > limit {
		return 0, nil, fmt.Errorf("frame of %d bytes exceeds limit", size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return header[0], payload, nil
}
