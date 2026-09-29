// source-health-sign signs a sensor's report using a separately managed key.
// It performs no health observation. Producers must supply independent evidence.
package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fortuna/api/collection"
)

func run() error {
	keyPath := flag.String("key", "", "Ed25519 PKCS#8 PEM private key file")
	flag.Parse()
	if *keyPath == "" {
		return fmt.Errorf("-key is required")
	}
	raw, err := os.ReadFile(*keyPath)
	if err != nil {
		return err
	}
	block, rest := pem.Decode(raw)
	if block == nil || len(rest) != 0 {
		return fmt.Errorf("expected one PKCS#8 PEM key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return fmt.Errorf("expected Ed25519 key")
	}
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 16385))
	decoder.DisallowUnknownFields()
	var report collection.RuntimeSourceHealth
	if err = decoder.Decode(&report); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing report data")
	}
	if err = report.Validate(time.Now().UTC()); err != nil {
		return err
	}
	payload, err := report.SigningBytes()
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(collection.SignedRuntimeSourceHealth{Report: report, Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))})
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
