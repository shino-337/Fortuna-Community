package extractor

import (
	"bytes"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// memLayer is a v1.Layer backed by in-memory uncompressed tar bytes.
type memLayer struct {
	diffID v1.Hash
	digest v1.Hash
	blob   []byte
}

func (m *memLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(m.blob)), nil
}

func (m *memLayer) DiffID() (v1.Hash, error) {
	return m.diffID, nil
}

func (m *memLayer) Digest() (v1.Hash, error) {
	return m.digest, nil
}

func (m *memLayer) Size() (int64, error) {
	return int64(len(m.blob)), nil
}

func (m *memLayer) MediaType() (types.MediaType, error) {
	return types.OCILayer, nil
}

func (m *memLayer) Compressed() (io.ReadCloser, error) {
	return m.Uncompressed()
}

// materializedImage is a v1.Image backed by in-memory config, manifest, and layers.
// Used after reading an image from a tarball so the tar file can be closed/removed.
type materializedImage struct {
	configFile  *v1.ConfigFile
	rawConfig   []byte
	digest      v1.Hash
	manifest    *v1.Manifest
	rawManifest []byte
	layers      []*memLayer
}

func (m *materializedImage) Layers() ([]v1.Layer, error) {
	out := make([]v1.Layer, len(m.layers))
	for i, l := range m.layers {
		out[i] = l
	}
	return out, nil
}

func (m *materializedImage) ConfigName() (v1.Hash, error) {
	if m.configFile == nil {
		return v1.Hash{}, nil
	}
	return v1.Hash{
		Algorithm: "sha256",
		Hex:       m.configFile.Config.Image,
	}, nil
}

func (m *materializedImage) ConfigFile() (*v1.ConfigFile, error) {
	return m.configFile, nil
}

func (m *materializedImage) RawConfigFile() ([]byte, error) {
	return m.rawConfig, nil
}

func (m *materializedImage) Digest() (v1.Hash, error) {
	return m.digest, nil
}

func (m *materializedImage) Manifest() (*v1.Manifest, error) {
	return m.manifest, nil
}

func (m *materializedImage) RawManifest() ([]byte, error) {
	return m.rawManifest, nil
}

func (m *materializedImage) Size() (int64, error) {
	var n int64
	for _, l := range m.layers {
		s, _ := l.Size()
		n += s
	}
	return n, nil
}

func (m *materializedImage) MediaType() (types.MediaType, error) {
	if m.manifest != nil && m.manifest.MediaType != "" {
		return m.manifest.MediaType, nil
	}
	return types.OCIManifestSchema1, nil
}

func (m *materializedImage) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	for _, l := range m.layers {
		if d, _ := l.DiffID(); d == h {
			return l, nil
		}
	}
	return nil, nil
}

func (m *materializedImage) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	for _, l := range m.layers {
		if d, _ := l.Digest(); d == h {
			return l, nil
		}
	}
	return nil, nil
}
