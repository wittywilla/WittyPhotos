// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wittywilla/wittymodule/service"
)

// call runs a handler as a server's service.Call reaches it: each argument written and read the
// way the wire does, and each result read back into what the server reads it into.
func call(t *testing.T, name string, out []any, args ...any) error {
	t.Helper()
	var raw []json.RawMessage
	for _, a := range args {
		b, err := service.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, b)
	}
	rets, _, err := Handlers[name](context.Background(), nil, raw)
	for i, v := range out {
		if i < len(rets) {
			b, merr := service.Marshal(rets[i])
			if merr != nil {
				t.Fatal(merr)
			}
			if uerr := service.Unmarshal(b, v); uerr != nil {
				t.Fatal(uerr)
			}
		}
	}
	return err
}

// serverInfo is the server's library.PhotoInfo, field for field: the wire carries Go field names.
type serverInfo struct {
	Taken                      time.Time
	HasOffset                  bool
	Make, Model, Lens          string
	FNumber                    float64
	Exposure                   string
	ISO                        int
	FocalMM                    float64
	Orientation, Width, Height int
	HasGPS                     bool
	Lat, Lon, Alt              float64
}

func TestCalls(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.png")
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 7, 3)))
	if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	var w, h int
	if err := call(t, "dimensions", []any{&w, &h}, p); err != nil || w != 7 || h != 3 {
		t.Fatalf("%v %dx%d", err, w, h)
	}
	var info serverInfo
	err := call(t, "read", []any{&info}, p)
	if ErrorCode(err) != "no-exif" {
		t.Fatalf("a PNG with no EXIF: %v (%q)", err, ErrorCode(err))
	}
	var jpg []byte
	if err := call(t, "preview", []any{&jpg}, p); ErrorCode(err) != "no-preview" {
		t.Fatalf("no preview: %v (%q)", err, ErrorCode(err))
	}
	if err := call(t, "read", nil); ErrorCode(err) != "bad-call" {
		t.Fatalf("no path: %v", err)
	}
	if err := call(t, "read", nil, filepath.Join(dir, "none.jpg")); err == nil || ErrorCode(err) != "" {
		t.Fatalf("missing file: %v %q", err, ErrorCode(err))
	}
}
