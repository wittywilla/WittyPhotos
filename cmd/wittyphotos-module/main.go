// SPDX-License-Identifier: AGPL-3.0-or-later
//
// WittyPhotos - Photo libraries: what a photo says about itself, including RAW files, without decoding it.
// Copyright (C) 2026 WittyWilla and contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Command wittyphotos-module is WittyPhotos as a helper process, so a new version
// can reach a server without a new server. See WittyModule for the protocol.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/wittywilla/wittymodule"
	"github.com/wittywilla/wittyphotos"
)

func main() { wittymodule.Main(wittyphotos.ManifestJSON, handle) }

// Dims is the answer to "dimensions".
type dims struct {
	W int `json:"w"`
	H int `json:"h"`
}

func handle(method string, params json.RawMessage) (any, []byte, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Path == "" {
		return nil, nil, &wittymodule.CallError{Code: "bad-call", Message: "no path was given"}
	}
	switch method {
	case "read":
		in, err := wittyphotos.ReadFile(p.Path)
		if err != nil {
			return nil, nil, coded(err)
		}
		return in, nil, nil
	case "dimensions":
		w, h := wittyphotos.Dimensions(p.Path)
		return dims{w, h}, nil, nil
	case "preview":
		b, err := wittyphotos.RawPreview(p.Path)
		if err != nil {
			return nil, nil, coded(err)
		}
		return nil, b, nil
	}
	return nil, nil, &wittymodule.CallError{Code: "unknown-method", Message: fmt.Sprintf("this version has no %q", method)}
}

// coded names the errors the server acts on.
func coded(err error) error {
	code := ""
	switch {
	case errors.Is(err, wittyphotos.ErrNoPreview):
		code = "no-preview"
	case errors.Is(err, wittyphotos.ErrNoExif):
		code = "no-exif"
	case errors.Is(err, fs.ErrPermission):
		code = "denied" // the helper's limits may be why, so the server tries itself
	}
	return &wittymodule.CallError{Code: code, Message: err.Error()}
}
