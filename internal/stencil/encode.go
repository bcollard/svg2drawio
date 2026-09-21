package stencil

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"fmt"
	"strings"
)

// StyleValue renders the stencil for use as the shape= entry of a draw.io cell
// style. draw.io decodes it with Graph.decompress: base64 -> raw inflate ->
// decodeURIComponent, so the shape XML is URI encoded before it is deflated.
func (s *Shape) StyleValue() (string, error) {
	data, err := Compress(s.XML(false))
	if err != nil {
		return "", err
	}
	return "stencil(" + data + ")", nil
}

// Compress applies draw.io's Graph.compress: encodeURIComponent, raw deflate,
// base64.
func Compress(xml string) (string, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return "", fmt.Errorf("deflate writer: %w", err)
	}
	if _, err := w.Write([]byte(encodeURIComponent(xml))); err != nil {
		return "", fmt.Errorf("deflate: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("deflate close: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Decompress reverses Compress. It exists so tests can round-trip a stencil
// the same way draw.io does.
func Decompress(data string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("base64: %w", err)
	}
	r := flate.NewReader(bytes.NewReader(raw))
	defer r.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		return "", fmt.Errorf("inflate: %w", err)
	}
	return decodeURIComponent(out.String())
}

// encodeURIComponent mirrors the JavaScript function of the same name: every
// byte outside the unreserved set is percent encoded.
func encodeURIComponent(s string) string {
	const unreserved = "-_.!~*'()"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			strings.IndexByte(unreserved, c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func decodeURIComponent(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			b.WriteByte(s[i])
			continue
		}
		if i+2 >= len(s) {
			return "", fmt.Errorf("truncated percent escape")
		}
		var v int
		if _, err := fmt.Sscanf(s[i+1:i+3], "%02x", &v); err != nil {
			return "", fmt.Errorf("bad percent escape %q: %w", s[i:i+3], err)
		}
		b.WriteByte(byte(v))
		i += 2
	}
	return b.String(), nil
}
