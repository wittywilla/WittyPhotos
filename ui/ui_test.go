// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package ui

import (
	"slices"
	"testing"

	"github.com/wittywilla/wittymodule/sdui"
	core "github.com/wittywilla/wittymodule/sdui/core"
	"github.com/wittywilla/wittymodule/sdui/sduitest"
)

// sdui.json is this definition, and it passes the kit's checks. Run with -update-sdui to
// write it again.
func TestSDUIFile(t *testing.T) {
	sduitest.Golden(t, UI(), "../sdui.json")
}

// The whole file, as the server reads and checks it.
func TestServerReadsIt(t *testing.T) {
	b, err := sdui.JSON(UI())
	if err != nil {
		t.Fatal(err)
	}
	m, err := core.ParseModuleUI(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range core.ValidateModuleUI(m, Module, core.ModuleChecks{ColorToken: func(s string) bool { return slices.Contains(sdui.ColorTokens, s) }}) {
		if is.Severity == "error" {
			t.Errorf("%s: %s", is.Path, is.Message)
		}
	}
	// The native apps draw the library's timeline themselves: the view keeps the app's own name.
	if v := m.Pages[0].Sections[0].Props["view"]; v != "photoTimeline" {
		t.Errorf("the library page's view is %v, not photoTimeline", v)
	}
	for _, id := range []string{PageLibrary, PagePhotos, PageAlbums, PageAlbum} {
		if !slices.ContainsFunc(m.Pages, func(p core.Page) bool { return p.ID == id }) {
			t.Errorf("no page %s", id)
		}
	}
}
