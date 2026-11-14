// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

// Package store is WittyPhotos' own database (module API 5): a SQLite file in the module's own
// folder (WITTYMODULE_DATA), with migrations WittyPhotos owns. It holds albums, and refers to the
// server's library, item and user ids as they are. It starts empty: nothing is brought over from a
// server's database.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// File is the database's name in the module's folder.
const File = "wittyphotos.db"

// ErrNotFound: no such album.
var ErrNotFound = errors.New("not found")

// migrations are the schema, in order; each runs once.
var migrations = []string{
	// 1: albums. An album belongs to the account that made it; a shared one is also seen by every
	// account that can open its library, which still sees only the pictures it may.
	`CREATE TABLE IF NOT EXISTS albums (
	   id INTEGER PRIMARY KEY,
	   library_id INTEGER NOT NULL,
	   owner_id INTEGER NOT NULL,
	   name TEXT NOT NULL,
	   description TEXT NOT NULL DEFAULT '',
	   shared INTEGER NOT NULL DEFAULT 0,
	   cover_item INTEGER,
	   created_at INTEGER NOT NULL,
	   updated_at INTEGER NOT NULL);
	 CREATE INDEX IF NOT EXISTS albums_lib ON albums(library_id, name);
	 CREATE INDEX IF NOT EXISTS albums_owner ON albums(owner_id);
	 CREATE TABLE IF NOT EXISTS album_items (
	   album_id INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
	   item_id INTEGER NOT NULL,
	   added_at INTEGER NOT NULL,
	   PRIMARY KEY(album_id, item_id));
	 CREATE INDEX IF NOT EXISTS album_items_item ON album_items(item_id);`,
}

// DB is WittyPhotos' database.
type DB struct{ *sql.DB }

// Open opens (and makes, or brings up to date) the database in dir.
func Open(dir string) (*DB, error) {
	if dir == "" {
		return nil, fmt.Errorf("no data folder (WITTYMODULE_DATA): WittyPhotos needs the data permission")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, File)+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	d := &DB{db}
	if err := d.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) migrate(ctx context.Context) error {
	if _, err := d.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema (version INTEGER NOT NULL)`); err != nil {
		return err
	}
	var v int
	if err := d.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema`).Scan(&v); err != nil {
		return err
	}
	for i := v; i < len(migrations); i++ {
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM schema`); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema(version) VALUES(?)`, i+1); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Now is the time rows are stamped with, in Unix seconds. Tests may change it.
var Now = func() int64 { return time.Now().Unix() }

// Album is one album's row.
type Album struct {
	ID          int64
	LibraryID   int64
	OwnerID     int64
	Name        string
	Description string
	Shared      bool
	Cover       int64 // 0: none chosen
	CreatedAt   int64
	UpdatedAt   int64
}

const albumCols = `id, library_id, owner_id, name, description, shared, IFNULL(cover_item, 0), created_at, updated_at`

func scanAlbum(sc interface{ Scan(...any) error }) (*Album, error) {
	var a Album
	if err := sc.Scan(&a.ID, &a.LibraryID, &a.OwnerID, &a.Name, &a.Description, &a.Shared, &a.Cover, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

// Albums lists the albums of a library an account sees: its own, and the shared ones, by name.
func (d *DB) Albums(ctx context.Context, library, user int64) ([]*Album, error) {
	rows, err := d.QueryContext(ctx, `SELECT `+albumCols+` FROM albums WHERE library_id = ? AND (owner_id = ? OR shared = 1)
		ORDER BY name COLLATE NOCASE LIMIT 2000`, library, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Album{}
	for rows.Next() {
		a, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Album loads one album: ErrNotFound when the account may not see it (not its own, not shared,
// and not an administrator).
func (d *DB) Album(ctx context.Context, id, user int64, admin bool) (*Album, error) {
	a, err := scanAlbum(d.QueryRowContext(ctx, `SELECT `+albumCols+` FROM albums WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !a.Shared && a.OwnerID != user && !admin {
		return nil, ErrNotFound
	}
	return a, nil
}

// CountOwned is how many albums an account has made.
func (d *DB) CountOwned(ctx context.Context, user int64) (int, error) {
	var n int
	err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM albums WHERE owner_id = ?`, user).Scan(&n)
	return n, err
}

// Create makes an album.
func (d *DB) Create(ctx context.Context, library, owner int64, name, description string, shared bool) (*Album, error) {
	now := Now()
	res, err := d.ExecContext(ctx, `INSERT INTO albums(library_id, owner_id, name, description, shared, created_at, updated_at) VALUES(?,?,?,?,?,?,?)`,
		library, owner, name, description, shared, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return d.Album(ctx, id, owner, true)
}

// Update changes an album's name, description and sharing, and its cover when cover isn't nil
// (0 goes back to the newest picture).
func (d *DB) Update(ctx context.Context, id int64, name, description string, shared bool, cover *int64) error {
	if cover != nil {
		var v any
		if *cover != 0 {
			v = *cover
		}
		if _, err := d.ExecContext(ctx, `UPDATE albums SET cover_item = ? WHERE id = ?`, v, id); err != nil {
			return err
		}
	}
	_, err := d.ExecContext(ctx, `UPDATE albums SET name = ?, description = ?, shared = ?, updated_at = ? WHERE id = ?`, name, description, shared, Now(), id)
	return err
}

// Delete removes an album; the pictures stay.
func (d *DB) Delete(ctx context.Context, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM albums WHERE id = ?`, id)
	return err
}

// Add puts items in an album, and says how many weren't there yet.
func (d *DB) Add(ctx context.Context, album int64, ids []int64) (int, error) {
	now, added := Now(), 0
	for _, id := range ids {
		res, err := d.ExecContext(ctx, `INSERT OR IGNORE INTO album_items(album_id, item_id, added_at) VALUES(?,?,?)`, album, id, now)
		if err != nil {
			return added, err
		}
		n, _ := res.RowsAffected()
		added += int(n)
	}
	_, err := d.ExecContext(ctx, `UPDATE albums SET updated_at = ? WHERE id = ?`, now, album)
	return added, err
}

// Remove takes items out of an album.
func (d *DB) Remove(ctx context.Context, album int64, ids []int64) error {
	for _, id := range ids {
		if _, err := d.ExecContext(ctx, `DELETE FROM album_items WHERE album_id = ? AND item_id = ?`, album, id); err != nil {
			return err
		}
	}
	return nil
}

// Items are an album's items, newest added first.
func (d *DB) Items(ctx context.Context, album int64) ([]int64, error) {
	rows, err := d.QueryContext(ctx, `SELECT item_id FROM album_items WHERE album_id = ? ORDER BY added_at DESC, rowid DESC`, album)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Holding are the albums, of those given, that hold an item.
func (d *DB) Holding(ctx context.Context, albums []int64, item int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(albums) == 0 {
		return out, nil
	}
	q := strings.TrimSuffix(strings.Repeat("?,", len(albums)), ",")
	args := []any{item}
	for _, a := range albums {
		args = append(args, a)
	}
	rows, err := d.QueryContext(ctx, `SELECT album_id FROM album_items WHERE item_id = ? AND album_id IN (`+q+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ForgetUser removes the albums an account made.
func (d *DB) ForgetUser(ctx context.Context, user int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM albums WHERE owner_id = ?`, user)
	return err
}

// ForgetLibrary removes the albums of a library that went.
func (d *DB) ForgetLibrary(ctx context.Context, library int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM albums WHERE library_id = ?`, library)
	return err
}
