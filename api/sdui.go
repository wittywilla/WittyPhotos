// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/wittywilla/wittymodule/host"
	"github.com/wittywilla/wittymodule/sdui"
	"github.com/wittywilla/wittyphotos/ui"
)

// The sources behind the albums on every app (sdui.json: the library and Photos pages, and
// wittyphotos-albums and wittyphotos-album), the Apple TV and Android TV apps among them, which
// have no web view. They show what the person may see and change nothing: albums are made and
// changed in a browser or the desktop app.

// SDUIItems answers the server's sdui.items call for WittyPhotos' sources.
func SDUIItems(req sdui.ItemsRequest) (sdui.ItemsAnswer, error) {
	if req.Viewer.User <= 0 {
		return sdui.ItemsAnswer{}, nil
	}
	a, err := Get()
	if err != nil {
		return sdui.ItemsAnswer{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return a.Items(ctx, req)
}

// refParam reads a source's parameter that names something by its number.
func refParam(src sdui.Source, name string) int64 {
	switch v := src[name].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

func photoImages(id int64) map[string]string {
	pic := "/api/items/" + strconv.FormatInt(id, 10) + "/image?w=480"
	return map[string]string{"square": pic, "poster": pic, "backdrop": pic}
}

// Items answers one source for the viewer.
func (a *App) Items(ctx context.Context, req sdui.ItemsRequest) (sdui.ItemsAnswer, error) {
	p := person{ID: req.Viewer.User, Admin: req.Viewer.Admin, Kind: req.Viewer.ProfileKind, Limited: len(req.Viewer.Cannot) > 0,
		on: host.On{User: req.Viewer.User}}
	var rows []sdui.Item
	switch req.Source.Type() {
	case ui.SourceAlbums:
		lib := refParam(req.Source, "library")
		if !a.photoLibrary(ctx, p, lib) {
			return sdui.ItemsAnswer{}, nil
		}
		list, err := a.DB.Albums(ctx, lib, p.ID)
		if err != nil {
			return sdui.ItemsAnswer{}, err
		}
		views, err := a.views(ctx, p, list)
		if err != nil {
			return sdui.ItemsAnswer{}, err
		}
		for _, v := range views {
			q := url.Values{"item": {strconv.FormatInt(v.ID, 10)}, "name": {v.Name}}
			it := sdui.Item{ID: "album-" + strconv.FormatInt(v.ID, 10), Kind: "collection", Title: v.Name, Count: v.Count,
				Subtitle: plural(v.Count, "picture", "pictures"), Overview: v.Description, Icon: "collection",
				Route: "/page/" + ui.PageAlbum + "?" + q.Encode()}
			if v.OwnerID != p.ID && v.Owner != "" {
				it.Subtitle += " · from " + v.Owner
			}
			if v.CoverID != 0 {
				it.Images = photoImages(v.CoverID)
			}
			rows = append(rows, it)
		}
	case ui.SourceAlbum:
		al, err := a.DB.Album(ctx, refParam(req.Source, "album"), p.ID, p.Admin)
		if err != nil || !a.photoLibrary(ctx, p, al.LibraryID) {
			return sdui.ItemsAnswer{}, nil
		}
		ids, err := a.albumItems(ctx, p, al)
		if err != nil {
			return sdui.ItemsAnswer{}, err
		}
		total := len(ids)
		start := min(max(req.Offset, 0), total)
		limit := req.Limit
		if limit <= 0 || limit > 200 {
			limit = 60
		}
		page := ids[start:min(total, start+limit)]
		titles := map[int64]string{}
		if len(page) > 0 {
			items, err := a.Host.Items(ctx, p.on, host.Query{Library: al.LibraryID, IDs: page, Limit: 500})
			if err != nil {
				return sdui.ItemsAnswer{}, err
			}
			for _, it := range items {
				titles[it.ID] = it.Title
			}
		}
		for _, id := range page {
			sid := strconv.FormatInt(id, 10)
			rows = append(rows, sdui.Item{ID: sid, Kind: "photo", Title: titles[id], Route: "/item/" + sid, Images: photoImages(id)})
		}
		return sdui.ItemsAnswer{Items: rows, Total: total, More: start+len(page) < total}, nil
	}
	return sdui.ItemsAnswer{Items: rows, Total: len(rows)}, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
