// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/wittywilla/wittymodule/host"
	"github.com/wittywilla/wittyphotos/ui"
)

// The photo map's online map: an administrator's settings (sdui.json, Settings, Photos). Without
// one the map draws the world's outline from the server and asks no one for anything.

// ValidMapTiles: empty (no online map), or an https address of map squares.
func ValidMapTiles(v string) bool {
	if v == "" {
		return true
	}
	return strings.HasPrefix(v, "https://") && strings.Contains(v, "{z}") && strings.Contains(v, "{x}") && strings.Contains(v, "{y}") && len(v) < 400 && !strings.ContainsAny(v, "\"'<> ")
}

// ValidMapCredit: one line of plain text, up to 200 characters.
func ValidMapCredit(v string) bool {
	return len(v) <= 200 && !strings.ContainsAny(v, "<>\r\n")
}

// MapSource is where the map's squares come from, and the credit its provider asks for.
type MapSource struct {
	Tiles       string `json:"tiles"`
	Attribution string `json:"attribution"`
}

// OnlineMap reads the settings: nothing while the online map is off or its address isn't one of map
// squares over https. A credit that isn't plain text is left out.
func OnlineMap(settings map[string]json.RawMessage) MapSource {
	var on bool
	var tiles, credit string
	_ = json.Unmarshal(settings[ui.SettingMapOnline], &on)
	_ = json.Unmarshal(settings[ui.SettingMapTiles], &tiles)
	_ = json.Unmarshal(settings[ui.SettingMapAttribution], &credit)
	tiles = strings.TrimSpace(tiles)
	if !on || tiles == "" || !ValidMapTiles(tiles) {
		return MapSource{}
	}
	if !ValidMapCredit(credit) {
		credit = ""
	}
	return MapSource{Tiles: tiles, Attribution: strings.TrimSpace(credit)}
}

func (a *App) onlineMap(ctx context.Context) (MapSource, error) {
	s, err := a.Host.Settings(ctx, host.On{})
	if err != nil {
		return MapSource{}, err
	}
	return OnlineMap(s), nil
}

// GET /map: {tiles, attribution}, both "" while there's no online map.
func (a *App) handleMap(w http.ResponseWriter, r *http.Request) {
	m, err := a.onlineMap(r.Context())
	if err != nil {
		a.hostErr(w, r, err)
		return
	}
	writeJSON(w, 200, m)
}
