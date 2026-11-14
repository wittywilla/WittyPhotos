// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

// Package ui is WittyPhotos' server-driven UI (module API 5; WittyMediaServer docs/SDUI-MODULES.md),
// written out as sdui.json: the page of every photo library and the Photos page, which every app
// draws (the Apple TV and Android TV apps among them, which have no web view), the albums pages
// for those apps, the map's settings, and the two views the web bundle draws in the web and
// desktop apps. Its sources are answered by WittyPhotos' service (package api).
package ui

import (
	"github.com/wittywilla/wittymodule/sdui"
)

// Module is this module's ID.
const Module = "wittyphotos"

// Pages.
const (
	PageLibrary = Module + "-library" // a photo library
	PagePhotos  = Module              // every photo library together
	PageAlbums  = Module + "-albums"  // a library's albums (?item=<library id>&name=<its name>)
	PageAlbum   = Module + "-album"   // one album's pictures (?item=<album id>&name=<its name>)
)

// Sources, answered by the service.
const (
	SourceAlbums = Module + ".albums" // the albums a person sees in a library, each opening its page
	SourceAlbum  = Module + ".album"  // the pictures in one album the person may see
)

// Views the web bundle draws (the web and desktop apps); other apps draw what's inside.
const (
	ViewTimeline = Module + ".timeline"
	ViewMap      = Module + ".map"
)

// Settings keys (the server's; read by the service with the host API's Settings).
const (
	SettingMapOnline      = "mapOnline"
	SettingMapTiles       = "mapTiles"
	SettingMapAttribution = "mapAttribution"
)

func sources() []sdui.SourceSpec {
	return []sdui.SourceSpec{
		{Type: SourceAlbums, Label: "Photo albums", Icon: "collection", Title: "Albums", Category: "library",
			Description: "The albums a person sees in a photo library: their own and the shared ones, each with its cover and how many pictures they may see in it, opening the album.",
			Params:      []sdui.PropSpec{{Name: "library", Type: "ref", Label: "Library", Required: true, Help: "The photo library's ID: $library on its page, $each over libraries."}},
			Yields:      []string{"collection"}, CardStyle: "photo"},
		{Type: SourceAlbum, Label: "An album's pictures", Icon: "photo", Title: "", Category: "library",
			Description: "The pictures in one album that the person may see, newest added first.",
			Params:      []sdui.PropSpec{{Name: "album", Type: "ref", Label: "Album", Required: true, Help: "The album's ID, $item on its page."}},
			Yields:      []string{"photo"}, CardStyle: "photo"},
	}
}

func recentEach() sdui.Section {
	s := sdui.Sec("recent", "Shelf", sdui.Props("title", "{each}", "cardStyle", "photo", "cardSize", "l", "showTitles", false, "seeAll", "always"), sdui.Src("recentlyAdded", "library", "$each"))
	s.Repeat = &sdui.Repeat{Over: "libraries", Kinds: []string{"photos"}}
	return s
}

// UI is the module's SDUI.
func UI() *sdui.Module {
	m := sdui.New(Module, "Photos")
	for _, s := range sources() {
		m.AddSource(s)
	}
	m.AddView(sdui.ModuleView{ID: ViewTimeline, Label: "Photo timeline",
		Description: "A photo library as the web and desktop apps draw it: the timeline by month, albums, favourites, folders, the map and uploads. Other apps show what's inside it.",
		Platforms:   []string{"web", "desktop"}})
	m.AddView(sdui.ModuleView{ID: ViewMap, Label: "Photo map",
		Description: "The pictures of a photo library that say where they were taken, on a map, in the web and desktop apps. Other apps show what's inside it.",
		Platforms:   []string{"web", "desktop"}})

	albums := sdui.Sec("albums", "Shelf", sdui.Props("title", "Albums", "cardStyle", "photo", "seeAll", "never", "empty", "hide"), sdui.Src(SourceAlbums, "library", "$library"))
	lib := sdui.Page{ID: PageLibrary, Kind: "library", Title: "{library}", Icon: "photo", Match: &sdui.Match{LibraryKind: "photos"},
		Description: "A photo library: the timeline with albums, favourites, folders, the map and people where the app has them; elsewhere its albums, then every photo, newest first.",
		Sections: []sdui.Section{sdui.Sec("timeline", "NativeView", sdui.Props("view", "photoTimeline", "library", "$library"), nil,
			albums,
			sdui.Sec("all", "Grid", sdui.Props("cardStyle", "photo", "showTitles", false, "empty", "message", "emptyText", "No photos yet. Put pictures in this library's folders and scan it."),
				sdui.Src("recentlyAdded", "library", "$library")))}}
	m.AddPage(lib)

	photos := sdui.Page{ID: PagePhotos, Kind: "photos", Title: "Photos", Icon: "photo",
		Description: "Every photo library together, newest first, then the albums in each.",
		Sections: []sdui.Section{recentEach(), func() sdui.Section {
			s := sdui.Sec("albums", "Shelf", sdui.Props("title", "Albums in {each}", "cardStyle", "photo", "seeAll", "never", "empty", "hide"), sdui.Src(SourceAlbums, "library", "$each"))
			s.Repeat = &sdui.Repeat{Over: "libraries", Kinds: []string{"photos"}}
			return s
		}()}}
	m.AddPage(photos)

	list := sdui.Page{ID: PageAlbums, Kind: "custom", Title: "{title}", Icon: "collection", Scope: Module,
		Description: "A photo library's albums on every app: the person's own and the shared ones, each opening its pictures. Opened with ?item=<library id>&name=<its name>.",
		Sections: []sdui.Section{sdui.Sec("albums", "Grid", sdui.Props("cardStyle", "photo", "empty", "message", "emptyText", "No albums yet. Albums are made in a browser or the desktop app."),
			sdui.Src(SourceAlbums, "library", "$item"))}}
	m.AddPage(list)

	one := sdui.Page{ID: PageAlbum, Kind: "custom", Title: "{title}", Icon: "photo", Scope: Module,
		Description: "One album's pictures on every app, newest added first, each opening as in its library. Opened with ?item=<album id>&name=<its name>.",
		Sections: []sdui.Section{sdui.Sec("photos", "Grid", sdui.Props("cardStyle", "photo", "showTitles", false, "empty", "message", "emptyText", "This album is empty, or holds nothing this profile may see."),
			sdui.Src(SourceAlbum, "album", "$item"))}}
	m.AddPage(one)

	m.AddPreset(sdui.ModulePreset{ID: Module + "-recent", Label: "New photos in each library",
		Description: "A row of large pictures for each photo library, newest first.", Section: func() sdui.Section {
			s := recentEach()
			s.ID = "photos"
			return s
		}()})
	m.AddNav(sdui.NavItem{ID: Module, Type: "page", Page: PagePhotos, Label: "Photos", Icon: "photo",
		When: &sdui.Condition{HasLibrary: "photos"}, Placement: map[string]string{"phone": "more"}})

	m.AddSettings(sdui.SettingsPage{ID: Module, Title: "Photos", Icon: "photo", Admin: true, Groups: []sdui.SettingsGroup{
		{Title: "The map", Footer: "Without an online map, the photo map draws the world's outline from this server and asks no one for anything. With one, each person's browser asks the map's provider for the squares it shows, so the provider sees roughly where they are looking.",
			Rows: []sdui.SettingRow{
				{Key: SettingMapOnline, Kind: "bool", Label: "Use an online map", Help: "Off: only the outline, and nothing leaves this server.", Default: false},
				{Key: SettingMapTiles, Kind: "string", Label: "Online map address", Help: "An https address of map squares with {z}, {x} and {y} in it, such as https://tile.example.org/{z}/{x}/{y}.png. Use a provider whose terms allow it.", Default: "", MaxLength: 400},
				{Key: SettingMapAttribution, Kind: "string", Label: "Credit to show on the map", Help: "The credit the provider asks for, as one line of plain text.", Default: "", MaxLength: 200},
			}},
	}})
	return m
}
