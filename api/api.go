// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

// Package api is WittyPhotos' own service (module API 5): the routes WittyMediaServer passes to it
// at /api/modules/wittyphotos/... once it has checked that someone is signed in (integration.json),
// the sources of its SDUI pages, the calls the server's scan makes (EXIF, sizes, RAW previews),
// and its lifecycle. Albums are kept in WittyPhotos' own database (package store); what a person
// may see is always asked of the server, through the host API, with that person's identity, so
// an album never shows anyone a picture they couldn't open in the library.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/wittywilla/wittymodule/host"
	"github.com/wittywilla/wittymodule/service"
	"github.com/wittywilla/wittyphotos/store"
)

// DataEnv is where the server says WittyPhotos' own folder is.
const DataEnv = "WITTYMODULE_DATA"

// HeaderLimited is "1" on a request from a profile with limits set (age, libraries, screen time).
const HeaderLimited = "X-Witty-Limited"

// LibraryKind is the kind of library WittyPhotos serves.
const LibraryKind = "photos"

// App is what the routes need: WittyPhotos' database and folder, and the way back to the server.
type App struct {
	DB   *store.DB
	Dir  string
	Host *host.Client
	Log  *slog.Logger
}

// NewApp is an App over a database, a folder and a host API client.
func NewApp(db *store.DB, dir string, h *host.Client) *App {
	return &App{DB: db, Dir: dir, Host: h, Log: slog.Default()}
}

var (
	appMu  sync.Mutex
	shared *App
)

// Get opens WittyPhotos' database and reaches the server the first time something needs them.
func Get() (*App, error) {
	appMu.Lock()
	defer appMu.Unlock()
	if shared != nil {
		return shared, nil
	}
	dir := os.Getenv(DataEnv)
	db, err := store.Open(dir)
	if err != nil {
		return nil, err
	}
	h, err := host.New()
	if err != nil {
		db.Close()
		return nil, err
	}
	shared = NewApp(db, dir, h)
	return shared, nil
}

// Use makes a the App the routes use, for tests.
func Use(a *App) {
	appMu.Lock()
	shared = a
	appMu.Unlock()
}

// Routes adds the routes to a service's mux (service.Config.HTTP).
func Routes(mux *http.ServeMux) {
	with := func(f func(*App, http.ResponseWriter, *http.Request)) http.Handler {
		return service.People(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a, err := Get()
			if err != nil {
				slog.Error("wittyphotos: its database or the server can't be reached", "cat", "library", "err", err)
				writeErr(w, 503, "WittyPhotos can't reach its data right now")
				return
			}
			f(a, w, r)
		}))
	}
	mux.Handle("GET /albums", with((*App).handleAlbums))
	mux.Handle("POST /albums", with((*App).handleCreate))
	mux.Handle("PUT /albums/{id}", with((*App).handleUpdate))
	mux.Handle("DELETE /albums/{id}", with((*App).handleDelete))
	mux.Handle("GET /albums/{id}/items", with((*App).handleItems))
	mux.Handle("POST /albums/{id}/items", with((*App).handleAdd))
	mux.Handle("DELETE /albums/{id}/items", with((*App).handleRemove))
	mux.Handle("GET /map", with((*App).handleMap))
}

// person is who is asking, as the server said.
type person struct {
	ID      int64
	Admin   bool
	Kind    string
	Limited bool
	on      host.On
}

func who(r *http.Request) person {
	id := service.Who(r)
	return person{ID: id.User, Admin: id.Admin, Kind: id.Kind, Limited: r.Header.Get(HeaderLimited) == "1", on: host.For(r)}
}

// canPost: albums are for grown-up accounts. Kids and Guests can look at shared ones.
func (p person) canPost() bool { return p.Admin || p.Kind == "adult" || p.Kind == "" }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// readJSON reads a request's body, of at most 1 MiB.
func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v)
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// hostErr answers what the server said about a call it refused, or failed.
func (a *App) hostErr(w http.ResponseWriter, r *http.Request, err error) {
	if host.IsNotFound(err) || host.IsForbidden(err) || errors.Is(err, store.ErrNotFound) {
		writeErr(w, 404, "not found")
		return
	}
	a.Log.WarnContext(r.Context(), "wittyphotos: a call failed", "cat", "library", "err", err)
	writeErr(w, 500, "WittyPhotos couldn't do that just now")
}

// photoLibrary says whether a library is a photo library the person may open: one of the server's
// photo libraries, and, for a profile with limits (which may hold it to some libraries), one the
// server shows it something of.
func (a *App) photoLibrary(ctx context.Context, p person, id int64) bool {
	if id <= 0 {
		return false
	}
	libs, err := a.Host.Libraries(ctx)
	if err != nil {
		return false
	}
	found := false
	for _, l := range libs {
		found = found || (l.ID == id && l.Kind == LibraryKind)
	}
	if !found {
		return false
	}
	if p.Limited && !p.Admin {
		items, err := a.Host.Items(ctx, p.on, host.Query{Library: id, Limit: 1})
		return err == nil && len(items) > 0
	}
	return true
}

// visible are the items of ids the person may see (in one library, when library isn't 0), as a set.
func (a *App) visible(ctx context.Context, p person, library int64, ids []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	for start := 0; start < len(ids); start += 500 {
		chunk := ids[start:min(len(ids), start+500)]
		items, err := a.Host.Items(ctx, p.on, host.Query{Library: library, IDs: chunk, Limit: 500})
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			out[it.ID] = true
		}
	}
	return out, nil
}
