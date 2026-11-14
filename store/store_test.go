// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package store

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestStore(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	a, err := d.Create(ctx, 1, 2, "Mine", "", false)
	if err != nil || a.OwnerID != 2 || a.LibraryID != 1 {
		t.Fatalf("%v %+v", err, a)
	}
	if _, err := d.Album(ctx, a.ID, 3, false); !errors.Is(err, ErrNotFound) {
		t.Fatal("someone else saw a private album")
	}
	if _, err := d.Album(ctx, a.ID, 3, true); err != nil {
		t.Fatal("an admin didn't")
	}
	if n, _ := d.Add(ctx, a.ID, []int64{5, 6, 5}); n != 2 {
		t.Fatalf("added %d", n)
	}
	if ids, _ := d.Items(ctx, a.ID); !slices.Equal(ids, []int64{6, 5}) {
		t.Fatalf("%v", ids)
	}
	if h, _ := d.Holding(ctx, []int64{a.ID, 99}, 6); !h[a.ID] || h[99] {
		t.Fatalf("%v", h)
	}
	cover := int64(5)
	if err := d.Update(ctx, a.ID, "Renamed", "d", true, &cover); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.Album(ctx, a.ID, 3, false); got.Name != "Renamed" || got.Cover != 5 || !got.Shared {
		t.Fatalf("%+v", got)
	}
	_ = d.Remove(ctx, a.ID, []int64{6})
	if n, _ := d.CountOwned(ctx, 2); n != 1 {
		t.Fatal(n)
	}
	d.Close()
	// Opened again, it's all there, and the schema isn't run twice.
	d, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if ids, _ := d.Items(ctx, a.ID); !slices.Equal(ids, []int64{5}) {
		t.Fatalf("%v", ids)
	}
	_ = d.Delete(ctx, a.ID)
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM album_items`).Scan(&n)
	if n != 0 {
		t.Fatal("an album's items stayed after it went")
	}
	if _, err := Open(""); err == nil {
		t.Fatal("no folder, no error")
	}
}
