// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/wittywilla/wittyphotos/store"
)

// Albums: a named set of pictures in one photo library. One belongs to the account that made it;
// a shared one is also seen by every account that can open its library, which still sees only the
// pictures its own limits allow.

// What one account can keep: far more than a person makes, and each row is a place to store things
// on someone else's disk. An administrator has no limit.
var maxAlbumsPerAccount = 200

const maxAlbumBatch = 2000

// Album is an album as the apps are given it.
type Album struct {
	ID          int64  `json:"id"`
	LibraryID   int64  `json:"libraryId"`
	OwnerID     int64  `json:"ownerId"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Shared      bool   `json:"shared"`
	Count       int    `json:"count"`
	CoverID     int64  `json:"coverId"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// badInput is a refusal in words for the person.
type badInput string

func (b badInput) Error() string { return string(b) }

type albumBody struct {
	LibraryID   int64  `json:"libraryId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Shared      bool   `json:"shared"`
	CoverID     *int64 `json:"coverId"`
}

func (b *albumBody) clean() error {
	b.Name, b.Description = strings.TrimSpace(b.Name), strings.TrimSpace(b.Description)
	if b.Name == "" || len(b.Name) > 200 {
		return badInput("an album needs a name of up to 200 characters")
	}
	if len(b.Description) > 4000 {
		return badInput("the description is too long")
	}
	return nil
}

// owners are account names by id, for the albums' "owner".
func (a *App) owners(ctx context.Context) map[int64]string {
	out := map[int64]string{}
	users, err := a.Host.Users(ctx)
	if err != nil {
		return out
	}
	for _, u := range users {
		out[u.ID] = u.Name
	}
	return out
}

// views are albums as the person sees them: counts and covers over the pictures they may see.
func (a *App) views(ctx context.Context, p person, list []*store.Album) ([]Album, error) {
	members := map[int64][]int64{}
	var all []int64
	seen := map[int64]bool{}
	for _, al := range list {
		ids, err := a.DB.Items(ctx, al.ID)
		if err != nil {
			return nil, err
		}
		members[al.ID] = ids
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				all = append(all, id)
			}
		}
		if al.Cover != 0 && !seen[al.Cover] {
			seen[al.Cover] = true
			all = append(all, al.Cover)
		}
	}
	vis, err := a.visible(ctx, p, 0, all)
	if err != nil {
		return nil, err
	}
	names := a.owners(ctx)
	out := make([]Album, 0, len(list))
	for _, al := range list {
		v := Album{ID: al.ID, LibraryID: al.LibraryID, OwnerID: al.OwnerID, Owner: names[al.OwnerID], Name: al.Name, Description: al.Description,
			Shared: al.Shared, CreatedAt: al.CreatedAt, UpdatedAt: al.UpdatedAt}
		for _, id := range members[al.ID] { // newest added first
			if vis[id] {
				if v.Count == 0 {
					v.CoverID = id
				}
				v.Count++
			}
		}
		if al.Cover != 0 && vis[al.Cover] {
			v.CoverID = al.Cover
		}
		out = append(out, v)
	}
	return out, nil
}

// GET /albums?library=[&item=]
func (a *App) handleAlbums(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	lib, _ := strconv.ParseInt(r.URL.Query().Get("library"), 10, 64)
	if !a.photoLibrary(r.Context(), p, lib) {
		writeErr(w, 404, "not found")
		return
	}
	list, err := a.DB.Albums(r.Context(), lib, p.ID)
	if err != nil {
		a.hostErr(w, r, err)
		return
	}
	if v := r.URL.Query().Get("item"); v != "" {
		item, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeErr(w, 400, "item is a picture's number")
			return
		}
		ids := make([]int64, len(list))
		for i, al := range list {
			ids[i] = al.ID
		}
		holding, err := a.DB.Holding(r.Context(), ids, item)
		if err != nil {
			a.hostErr(w, r, err)
			return
		}
		kept := list[:0]
		for _, al := range list {
			if holding[al.ID] {
				kept = append(kept, al)
			}
		}
		list = kept
	}
	out, err := a.views(r.Context(), p, list)
	if err != nil {
		a.hostErr(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

// POST /albums {libraryId, name, description, shared}
func (a *App) handleCreate(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	if !p.canPost() {
		writeErr(w, 403, "this profile can't make albums")
		return
	}
	if !p.Admin {
		n, err := a.DB.CountOwned(r.Context(), p.ID)
		if err != nil {
			writeErr(w, 500, "could not check that")
			return
		}
		if n >= maxAlbumsPerAccount {
			writeErr(w, 409, fmt.Sprintf("You have %d albums, which is as many as one account can have. Remove some first.", n))
			return
		}
	}
	var b albumBody
	if err := readJSON(w, r, &b); err != nil {
		writeErr(w, 400, "the request isn't readable")
		return
	}
	if err := b.clean(); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if !a.photoLibrary(r.Context(), p, b.LibraryID) {
		writeErr(w, 404, "not found")
		return
	}
	al, err := a.DB.Create(r.Context(), b.LibraryID, p.ID, b.Name, b.Description, b.Shared)
	if err != nil {
		a.hostErr(w, r, err)
		return
	}
	out, err := a.views(r.Context(), p, []*store.Album{al})
	if err != nil {
		a.hostErr(w, r, err)
		return
	}
	writeJSON(w, 201, out[0])
}

// ownAlbum loads an album the person may change: their own, or any for an administrator.
func (a *App) ownAlbum(w http.ResponseWriter, r *http.Request, p person) *store.Album {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, 404, "not found")
		return nil
	}
	if !p.canPost() {
		writeErr(w, 403, "this profile can't change albums")
		return nil
	}
	al, err := a.DB.Album(r.Context(), id, p.ID, p.Admin)
	if err != nil || (al.OwnerID != p.ID && !p.Admin) {
		writeErr(w, 404, "not found")
		return nil
	}
	return al
}

// PUT /albums/{id} {name, description, shared, coverId}
func (a *App) handleUpdate(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	al := a.ownAlbum(w, r, p)
	if al == nil {
		return
	}
	var b albumBody
	if err := readJSON(w, r, &b); err != nil {
		writeErr(w, 400, "the request isn't readable")
		return
	}
	if err := b.clean(); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if b.CoverID != nil && *b.CoverID != 0 {
		it, err := a.Host.Item(r.Context(), p.on, *b.CoverID)
		if err != nil || it.Library != al.LibraryID {
			writeErr(w, 400, "that picture can't be the cover")
			return
		}
	}
	if err := a.DB.Update(r.Context(), al.ID, b.Name, b.Description, b.Shared, b.CoverID); err != nil {
		a.hostErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// DELETE /albums/{id}: the pictures stay.
func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	al := a.ownAlbum(w, r, p)
	if al == nil {
		return
	}
	if err := a.DB.Delete(r.Context(), al.ID); err != nil {
		a.hostErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func readIDs(w http.ResponseWriter, r *http.Request) ([]int64, bool) {
	var b struct {
		IDs []int64 `json:"ids"`
	}
	if err := readJSON(w, r, &b); err != nil || len(b.IDs) == 0 || len(b.IDs) > maxAlbumBatch {
		writeErr(w, 400, "ids must list 1 to 2000 pictures")
		return nil, false
	}
	return b.IDs, true
}

// POST /albums/{id}/items {ids}: only pictures the person may see, of the album's library.
func (a *App) handleAdd(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	al := a.ownAlbum(w, r, p)
	if al == nil {
		return
	}
	ids, ok := readIDs(w, r)
	if !ok {
		return
	}
	vis, err := a.visible(r.Context(), p, al.LibraryID, ids)
	if err != nil {
		a.hostErr(w, r, err)
		return
	}
	allowed := make([]int64, 0, len(ids))
	for _, id := range ids {
		if vis[id] {
			allowed = append(allowed, id)
		}
	}
	n, err := a.DB.Add(r.Context(), al.ID, allowed)
	if err != nil {
		a.hostErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"added": n})
}

// DELETE /albums/{id}/items {ids}
func (a *App) handleRemove(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	al := a.ownAlbum(w, r, p)
	if al == nil {
		return
	}
	ids, ok := readIDs(w, r)
	if !ok {
		return
	}
	if err := a.DB.Remove(r.Context(), al.ID, ids); err != nil {
		a.hostErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// albumItems are the pictures of an album the person may see, newest added first.
func (a *App) albumItems(ctx context.Context, p person, al *store.Album) ([]int64, error) {
	ids, err := a.DB.Items(ctx, al.ID)
	if err != nil {
		return nil, err
	}
	vis, err := a.visible(ctx, p, al.LibraryID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if vis[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// GET /albums/{id}/items: {ids}, for the server's photo lists (/api/photos/list?ids=).
func (a *App) handleItems(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	id, ok := pathID(r)
	if !ok {
		writeErr(w, 404, "not found")
		return
	}
	al, err := a.DB.Album(r.Context(), id, p.ID, p.Admin)
	if err != nil || !a.photoLibrary(r.Context(), p, al.LibraryID) {
		writeErr(w, 404, "not found")
		return
	}
	ids, err := a.albumItems(r.Context(), p, al)
	if err != nil {
		a.hostErr(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ids": ids})
}
