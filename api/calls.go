// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"

	"github.com/wittywilla/wittymodule/service"
	"github.com/wittywilla/wittyphotos"
)

// The calls a server's scan makes on the service (service.Call): each takes the file's path, which
// the server chose, and only reads it.
//
//	read        args [path]  rets [wittyphotos.Info]   what the photo's EXIF says
//	dimensions  args [path]  rets [w, h]               its size from its header (0, 0 when unknown)
//	preview     args [path]  rets [[]byte]             the JPEG a camera stored in a RAW file
//
// A refusal carries a code the server acts on (ErrorCode).

// Handlers are the calls.
var Handlers = map[string]service.Handler{
	"read": func(_ context.Context, _ json.RawMessage, args []json.RawMessage) ([]any, any, error) {
		path, err := pathArg(args)
		if err != nil {
			return nil, nil, err
		}
		in, err := wittyphotos.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		return []any{in}, nil, nil
	},
	"dimensions": func(_ context.Context, _ json.RawMessage, args []json.RawMessage) ([]any, any, error) {
		path, err := pathArg(args)
		if err != nil {
			return nil, nil, err
		}
		w, h := wittyphotos.Dimensions(path)
		return []any{w, h}, nil, nil
	},
	"preview": func(_ context.Context, _ json.RawMessage, args []json.RawMessage) ([]any, any, error) {
		path, err := pathArg(args)
		if err != nil {
			return nil, nil, err
		}
		b, err := wittyphotos.RawPreview(path)
		if err != nil {
			return nil, nil, err
		}
		return []any{b}, nil, nil
	},
}

// errNoPath: a call without a file.
var errNoPath = errors.New("no path was given")

func pathArg(args []json.RawMessage) (string, error) {
	var p string
	if err := service.Arg(args, 0, &p); err != nil {
		return "", err
	}
	if p == "" {
		return "", errNoPath
	}
	return p, nil
}

// ErrorCode names the errors the server acts on.
func ErrorCode(err error) string {
	switch {
	case errors.Is(err, wittyphotos.ErrNoPreview):
		return "no-preview"
	case errors.Is(err, wittyphotos.ErrNoExif):
		return "no-exif"
	case errors.Is(err, fs.ErrPermission):
		return "denied" // the service's limits may be why, so the server tries itself
	case errors.Is(err, errNoPath):
		return "bad-call"
	}
	return ""
}
