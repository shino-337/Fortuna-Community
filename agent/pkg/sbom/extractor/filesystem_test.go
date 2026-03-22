package extractor

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"testing"
)

func makeSingleFileTar(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	h := &tar.Header{
		Name: name,
		Mode: 0600,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(h); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestFilesystem_ExtractTar_RespectsMaxFileBytes(t *testing.T) {
	fs := NewFilesystem()
	fs.maxFileBytes = 4
	fs.maxTotalBytes = 1024

	tarData := makeSingleFileTar(t, "big.bin", []byte("12345"))
	if err := fs.ExtractTar(context.Background(), 0, bytes.NewReader(tarData)); err != nil {
		t.Fatal(err)
	}
	if fs.FileExists("/big.bin") {
		t.Fatalf("expected big.bin to be skipped due to maxFileBytes")
	}
}

func TestFilesystem_ExtractTar_SkipPathPrefixes(t *testing.T) {
	fs := NewFilesystem()
	fs.skipPathPrefixes = []string{"/usr/share/doc"}
	tarData := makeSingleFileTar(t, "usr/share/doc/readme.txt", []byte("hello"))
	if err := fs.ExtractTar(context.Background(), 0, bytes.NewReader(tarData)); err != nil {
		t.Fatal(err)
	}
	if fs.FileExists("/usr/share/doc/readme.txt") {
		t.Fatal("expected file under skip prefix to not be materialized")
	}
	tarData2 := makeSingleFileTar(t, "app/Gemfile.lock", []byte("GEM\n"))
	if err := fs.ExtractTar(context.Background(), 0, bytes.NewReader(tarData2)); err != nil {
		t.Fatal(err)
	}
	if !fs.FileExists("/app/Gemfile.lock") {
		t.Fatal("expected Gemfile.lock to be stored")
	}
}

func TestFilesystem_ExtractTar_RespectsMaxTotalBytes(t *testing.T) {
	fs := NewFilesystem()
	fs.maxFileBytes = 1024
	fs.maxTotalBytes = 4

	tarData := makeSingleFileTar(t, "a.bin", []byte("12345"))
	if err := fs.ExtractTar(context.Background(), 0, bytes.NewReader(tarData)); err != nil {
		t.Fatal(err)
	}
	if fs.FileExists("/a.bin") {
		t.Fatalf("expected a.bin to be skipped due to maxTotalBytes")
	}
}

func TestFilesystem_IndexedMode_DiscoveryWithoutMaterialize(t *testing.T) {
	t.Setenv(envFSMode, fsModeIndexed)
	fs := NewFilesystem()
	t.Cleanup(func() { _ = fs.Close() })
	if fs.Mode() != fsModeIndexed {
		t.Fatalf("expected indexed mode, got %q", fs.Mode())
	}

	tarData := makeSingleFileTar(t, "var/lib/huge.bin", []byte("huge-content-not-needed-for-discovery"))
	if err := fs.ExtractTar(context.Background(), 0, bytes.NewReader(tarData)); err != nil {
		t.Fatal(err)
	}
	if !fs.FileExists("/var/lib/huge.bin") {
		t.Fatal("expected path in index for non-materialized file")
	}
	b, err := fs.ReadFile("/var/lib/huge.bin")
	if err != nil {
		t.Fatalf("ReadFile (lazy spool): %v", err)
	}
	if string(b) != "huge-content-not-needed-for-discovery" {
		t.Fatalf("unexpected lazy read: %q", b)
	}
	if len(fs.files) != 0 {
		t.Fatalf("expected no materialized files in RAM, got %d", len(fs.files))
	}
}

func TestFilesystem_IndexedMode_MaterializesSelectivePaths(t *testing.T) {
	t.Setenv(envFSMode, fsModeIndexed)
	fs := NewFilesystem()
	t.Cleanup(func() { _ = fs.Close() })

	tarData := makeSingleFileTar(t, "app/package.json", []byte(`{"name":"x"}`))
	if err := fs.ExtractTar(context.Background(), 0, bytes.NewReader(tarData)); err != nil {
		t.Fatal(err)
	}
	if !fs.FileExists("/app/package.json") {
		t.Fatal("expected package.json in index")
	}
	b, err := fs.ReadFile("/app/package.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"name":"x"}` {
		t.Fatalf("unexpected content: %s", b)
	}
}

func TestFilesystem_IndexedMode_WhiteoutRemovesFromIndex(t *testing.T) {
	t.Setenv(envFSMode, fsModeIndexed)
	fs := NewFilesystem()
	t.Cleanup(func() { _ = fs.Close() })

	layer1 := makeSingleFileTar(t, "opt/app/data.txt", []byte("v1"))
	if err := fs.ExtractTar(context.Background(), 0, bytes.NewReader(layer1)); err != nil {
		t.Fatal(err)
	}
	if !fs.FileExists("/opt/app/data.txt") {
		t.Fatal("expected file after layer1")
	}

	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	wh := &tar.Header{Name: "opt/app/.wh.data.txt", Mode: 0600, Size: 0}
	if err := tw.WriteHeader(wh); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := fs.ExtractTar(context.Background(), 1, bytes.NewReader(b.Bytes())); err != nil {
		t.Fatal(err)
	}
	if fs.FileExists("/opt/app/data.txt") {
		t.Fatal("expected whiteout to remove path from index")
	}
}

func TestFilesystem_MaterializeMode_UnsetEnv(t *testing.T) {
	_ = os.Unsetenv(envFSMode)
	fs := NewFilesystem()
	if fs.Mode() != fsModeMaterialize {
		t.Fatalf("expected default materialize, got %q", fs.Mode())
	}
}

func TestFilesystem_MetricsSnapshot_LazyReadCounts(t *testing.T) {
	t.Setenv(envFSMode, fsModeIndexed)
	fs := NewFilesystem()
	t.Cleanup(func() { _ = fs.Close() })

	tarData := makeSingleFileTar(t, "opt/x.bin", []byte("abc"))
	if err := fs.ExtractTar(context.Background(), 0, bytes.NewReader(tarData)); err != nil {
		t.Fatal(err)
	}
	if m := fs.MetricsSnapshot(); m.LazyReadOps != 0 || m.LazyReadBytes != 0 {
		t.Fatalf("before ReadFile: %+v", m)
	}
	if _, err := fs.ReadFile("/opt/x.bin"); err != nil {
		t.Fatal(err)
	}
	m := fs.MetricsSnapshot()
	if m.LazyReadOps != 1 || m.LazyReadBytes != 3 {
		t.Fatalf("after one read: %+v", m)
	}
	if _, err := fs.ReadFile("/opt/x.bin"); err != nil {
		t.Fatal(err)
	}
	m = fs.MetricsSnapshot()
	if m.LazyReadOps != 2 || m.LazyReadBytes != 6 {
		t.Fatalf("after two reads: %+v", m)
	}
}

func TestFormatBytesIEC(t *testing.T) {
	if formatBytesIEC(500) != "500 B" {
		t.Fatalf("500 B: %q", formatBytesIEC(500))
	}
	if formatBytesIEC(2048) != "2.00 KiB" {
		t.Fatalf("2 KiB: %q", formatBytesIEC(2048))
	}
}

