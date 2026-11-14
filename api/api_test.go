// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/wittywilla/wittymodule"
	"github.com/wittywilla/wittymodule/host"
	"github.com/wittywilla/wittymodule/sdui"
	"github.com/wittywilla/wittyphotos/store"
	"github.com/wittywilla/wittyphotos/ui"
)

// fakeServer stands in for the server's host API: two photo libraries (1 and 2) and a films
// library (9), the items in them, who may see which, the accounts, and the module's settings.
type fakeServer struct {
	mu       sync.Mutex
	settings map[string]any
	items    map[int64]int64          // item → library
	hidden   map[int64]map[int64]bool // user → items they may not see
	libsOf   map[int64]map[int64]bool // user → the only libraries they may open (nil: all)
	calls    int
}

func (f *fakeServer) sees(user, item int64) bool {
	lib, ok := f.items[item]
	if !ok || lib == 9 {
		return false // films aren't WittyPhotos' to see
	}
	if f.hidden[user][item] {
		return false
	}
	if only := f.libsOf[user]; only != nil && !only[lib] {
		return false
	}
	return true
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	w.Header().Set("X-Witty-Host", "1")
	user, _ := strconv.ParseInt(r.Header.Get(host.HeaderOnUser), 10, 64)
	switch {
	case r.URL.Path == host.Base+"/settings":
		_ = json.NewEncoder(w).Encode(f.settings)
	case r.URL.Path == host.Base+"/libraries":
		_, _ = w.Write([]byte(`{"libraries":[{"id":1,"name":"Family","kind":"photos","paths":[]},{"id":2,"name":"Work","kind":"photos","paths":[]}]}`))
	case r.URL.Path == host.Base+"/users":
		_, _ = w.Write([]byte(`{"users":[{"id":1,"name":"admin"},{"id":2,"name":"ann"},{"id":3,"name":"bob"},{"id":4,"name":"kid"}]}`))
	case r.URL.Path == host.Base+"/events":
		_, _ = w.Write([]byte(`{"events":[],"next":0}`))
	case strings.HasPrefix(r.URL.Path, host.Base+"/items/"):
		id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, host.Base+"/items/"), 10, 64)
		if _, ok := f.items[id]; !ok || f.items[id] == 9 {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"no such item"}`))
			return
		}
		if user > 0 && !f.sees(user, id) {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":"not for this person"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "library": f.items[id], "kind": "photo", "title": fmt.Sprintf("IMG_%d", id)})
	case r.URL.Path == host.Base+"/items":
		q := r.URL.Query()
		lib, _ := strconv.ParseInt(q.Get("library"), 10, 64)
		var ids []int64
		for _, s := range strings.Split(q.Get("ids"), ",") {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				ids = append(ids, n)
			}
		}
		if len(ids) > 500 {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"ids are up to 500"}`))
			return
		}
		if q.Get("ids") == "" {
			for id := range f.items {
				ids = append(ids, id)
			}
			slices.Sort(ids)
		}
		out := []map[string]any{}
		for _, id := range ids {
			if (lib == 0 || f.items[id] == lib) && f.sees(user, id) {
				out = append(out, map[string]any{"id": id, "library": f.items[id], "kind": "photo", "title": fmt.Sprintf("IMG_%d", id)})
			}
		}
		if l, _ := strconv.Atoi(q.Get("limit")); l > 0 && len(out) > l {
			out = out[:l]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
	default:
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":"no such call"}`))
	}
}

type testApp struct {
	*App
	srv *fakeServer
	mux *http.ServeMux
	dir string
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	srv := &fakeServer{settings: map[string]any{}, items: map[int64]int64{}, hidden: map[int64]map[int64]bool{}, libsOf: map[int64]map[int64]bool{}}
	for i := int64(1); i <= 1200; i++ {
		srv.items[i] = 1
	}
	for i := int64(2001); i <= 2010; i++ {
		srv.items[i] = 2
	}
	srv.items[9001] = 9
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	a := NewApp(db, dir, host.NewAt(hs.URL, "token", nil))
	Use(a)
	t.Cleanup(func() { Use(nil) })
	mux := http.NewServeMux()
	Routes(mux)
	return &testApp{App: a, srv: srv, mux: mux, dir: dir}
}

type who_ struct {
	id      int64
	admin   bool
	kind    string
	limited bool
}

var (
	admin = who_{id: 1, admin: true, kind: "adult"}
	ann   = who_{id: 2, kind: "adult"}
	bob   = who_{id: 3, kind: "adult"}
	kid   = who_{id: 4, kind: "kids"}
	guest = who_{id: 5, kind: "guest"}
)

func (ta *testApp) do(t *testing.T, method, path string, u who_, body any) *httptest.ResponseRecorder {
	t.Helper()
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if u.id > 0 {
		r.Header.Set("X-Witty-User", strconv.FormatInt(u.id, 10))
	}
	if u.admin {
		r.Header.Set("X-Witty-Admin", "1")
	}
	r.Header.Set("X-Witty-Kind", u.kind)
	if u.limited {
		r.Header.Set(HeaderLimited, "1")
	}
	w := httptest.NewRecorder()
	ta.mux.ServeHTTP(w, r)
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("%d %s: %v", w.Code, w.Body, err)
	}
	return v
}

func (ta *testApp) make(t *testing.T, u who_, lib int64, name string, shared bool) Album {
	t.Helper()
	w := ta.do(t, "POST", "/albums", u, map[string]any{"libraryId": lib, "name": name, "shared": shared})
	if w.Code != 201 {
		t.Fatalf("make %s: %d %s", name, w.Code, w.Body)
	}
	return decode[Album](t, w)
}

func TestNobodySignedIn(t *testing.T) {
	ta := newTestApp(t)
	if w := ta.do(t, "GET", "/albums?library=1", who_{}, nil); w.Code != 401 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestAlbumsMadeAndSeen(t *testing.T) {
	ta := newTestApp(t)
	mine := ta.make(t, ann, 1, "  Summer  ", false)
	if mine.Name != "Summer" || mine.Owner != "ann" || mine.LibraryID != 1 || mine.Count != 0 {
		t.Fatalf("%+v", mine)
	}
	shared := ta.make(t, ann, 1, "Shared", true)
	ta.make(t, ann, 2, "Elsewhere", true)
	// Bob sees the shared one only; ann both; the admin as before: own or shared.
	got := decode[[]Album](t, ta.do(t, "GET", "/albums?library=1", bob, nil))
	if len(got) != 1 || got[0].ID != shared.ID || got[0].Owner != "ann" {
		t.Fatalf("bob: %+v", got)
	}
	if got := decode[[]Album](t, ta.do(t, "GET", "/albums?library=1", ann, nil)); len(got) != 2 {
		t.Fatalf("ann: %+v", got)
	}
	if got := decode[[]Album](t, ta.do(t, "GET", "/albums?library=1", admin, nil)); len(got) != 1 {
		t.Fatalf("admin: %+v", got)
	}
	// Not a photo library, or none at all.
	for _, lib := range []string{"9", "0", "x", "77"} {
		if w := ta.do(t, "GET", "/albums?library="+lib, ann, nil); w.Code != 404 {
			t.Errorf("library %s: %d", lib, w.Code)
		}
		if w := ta.do(t, "POST", "/albums", ann, map[string]any{"libraryId": lib, "name": "x"}); w.Code != 404 && w.Code != 400 {
			t.Errorf("make in %s: %d", lib, w.Code)
		}
	}
	// A profile held to library 2 can't see library 1's albums.
	ta.srv.libsOf[6] = map[int64]bool{2: true}
	lim := who_{id: 6, kind: "adult", limited: true}
	if w := ta.do(t, "GET", "/albums?library=1", lim, nil); w.Code != 404 {
		t.Errorf("limited: %d", w.Code)
	}
	if w := ta.do(t, "GET", "/albums?library=2", lim, nil); w.Code != 200 {
		t.Errorf("limited, its own library: %d", w.Code)
	}
}

func TestNamesAndLimits(t *testing.T) {
	ta := newTestApp(t)
	for _, body := range []map[string]any{{"libraryId": 1, "name": "  "}, {"libraryId": 1, "name": strings.Repeat("a", 201)}, {"libraryId": 1, "name": "a", "description": strings.Repeat("d", 4001)}} {
		if w := ta.do(t, "POST", "/albums", ann, body); w.Code != 400 {
			t.Errorf("%v: %d", body["name"], w.Code)
		}
	}
	if w := ta.do(t, "POST", "/albums", ann, "not an album"); w.Code != 400 {
		t.Errorf("unreadable: %d", w.Code)
	}
	old := maxAlbumsPerAccount
	maxAlbumsPerAccount = 2
	t.Cleanup(func() { maxAlbumsPerAccount = old })
	ta.make(t, ann, 1, "a", false)
	ta.make(t, ann, 1, "b", false)
	w := ta.do(t, "POST", "/albums", ann, map[string]any{"libraryId": 1, "name": "c"})
	if w.Code != 409 || !strings.Contains(w.Body.String(), "You have 2 albums, which is as many as one account can have. Remove some first.") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	ta.make(t, admin, 1, "a", false)
	ta.make(t, admin, 1, "b", false)
	ta.make(t, admin, 1, "c", false) // an administrator has no limit
}

func TestKidsAndGuestsOnlyLook(t *testing.T) {
	ta := newTestApp(t)
	al := ta.make(t, ann, 1, "Shared", true)
	ta.do(t, "POST", fmt.Sprintf("/albums/%d/items", al.ID), ann, map[string]any{"ids": []int64{1, 2}})
	for _, u := range []who_{kid, guest} {
		if w := ta.do(t, "POST", "/albums", u, map[string]any{"libraryId": 1, "name": "mine"}); w.Code != 403 {
			t.Errorf("%s makes: %d", u.kind, w.Code)
		}
		for _, c := range []struct{ m, p string }{{"PUT", ""}, {"DELETE", ""}, {"POST", "/items"}, {"DELETE", "/items"}} {
			if w := ta.do(t, c.m, fmt.Sprintf("/albums/%d%s", al.ID, c.p), u, map[string]any{"name": "x", "ids": []int64{1}}); w.Code != 403 {
				t.Errorf("%s %s %s: %d", u.kind, c.m, c.p, w.Code)
			}
		}
		if got := decode[[]Album](t, ta.do(t, "GET", "/albums?library=1", u, nil)); len(got) != 1 || got[0].Count != 2 {
			t.Errorf("%s sees: %+v", u.kind, got)
		}
		if got := decode[map[string][]int64](t, ta.do(t, "GET", fmt.Sprintf("/albums/%d/items", al.ID), u, nil)); len(got["ids"]) != 2 {
			t.Errorf("%s items: %v", u.kind, got)
		}
	}
}

func TestOwnerOrAdminChanges(t *testing.T) {
	ta := newTestApp(t)
	al := ta.make(t, ann, 1, "Ann's", true)
	path := fmt.Sprintf("/albums/%d", al.ID)
	for _, c := range []struct {
		m, p string
		body any
	}{{"PUT", "", map[string]any{"name": "Bob's now"}}, {"POST", "/items", map[string]any{"ids": []int64{1}}}, {"DELETE", "/items", map[string]any{"ids": []int64{1}}}, {"DELETE", "", nil}} {
		if w := ta.do(t, c.m, path+c.p, bob, c.body); w.Code != 404 {
			t.Errorf("bob %s %s: %d", c.m, c.p, w.Code)
		}
	}
	if w := ta.do(t, "PUT", path, admin, map[string]any{"name": "Renamed", "description": "by the admin", "shared": false}); w.Code != 200 {
		t.Fatalf("admin: %d %s", w.Code, w.Body)
	}
	// Private now: bob doesn't see it at all.
	if got := decode[[]Album](t, ta.do(t, "GET", "/albums?library=1", bob, nil)); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	if w := ta.do(t, "GET", path+"/items", bob, nil); w.Code != 404 {
		t.Errorf("bob items of a private album: %d", w.Code)
	}
	got := decode[[]Album](t, ta.do(t, "GET", "/albums?library=1", ann, nil))
	if got[0].Name != "Renamed" || got[0].Description != "by the admin" || got[0].Shared {
		t.Fatalf("%+v", got[0])
	}
	if w := ta.do(t, "DELETE", path, ann, nil); w.Code != 200 {
		t.Fatalf("%d", w.Code)
	}
	if w := ta.do(t, "PUT", path, ann, map[string]any{"name": "x"}); w.Code != 404 {
		t.Errorf("gone: %d", w.Code)
	}
	if w := ta.do(t, "PUT", "/albums/x", ann, map[string]any{"name": "x"}); w.Code != 404 {
		t.Errorf("bad id: %d", w.Code)
	}
}

func TestItemsVisibleOnly(t *testing.T) {
	ta := newTestApp(t)
	al := ta.make(t, ann, 1, "Big", true)
	items := fmt.Sprintf("/albums/%d/items", al.ID)
	// More than one host call's worth (500), with pictures of another library, films and nothing.
	var ids []int64
	for i := int64(1); i <= 1100; i++ {
		ids = append(ids, i)
	}
	ids = append(ids, 2001, 9001, 424242)
	ta.srv.hidden[2] = map[int64]bool{7: true} // ann may not see 7
	w := ta.do(t, "POST", items, ann, map[string]any{"ids": ids})
	if got := decode[map[string]int](t, w); got["added"] != 1099 {
		t.Fatalf("added %v", got)
	}
	if got := decode[map[string]int](t, ta.do(t, "POST", items, ann, map[string]any{"ids": []int64{1, 2}})); got["added"] != 0 {
		t.Fatalf("again: %v", got)
	}
	// Bob may not see 10; the album shows him one fewer, and never 10.
	ta.srv.hidden[3] = map[int64]bool{10: true}
	got := decode[map[string][]int64](t, ta.do(t, "GET", items, bob, nil))
	if len(got["ids"]) != 1098 || slices.Contains(got["ids"], 10) {
		t.Fatalf("bob sees %d", len(got["ids"]))
	}
	list := decode[[]Album](t, ta.do(t, "GET", "/albums?library=1", bob, nil))
	if list[0].Count != 1098 {
		t.Fatalf("count %d", list[0].Count)
	}
	for _, bad := range []any{map[string]any{"ids": []int64{}}, map[string]any{"ids": make([]int64, 2001)}, "x"} {
		if w := ta.do(t, "POST", items, ann, bad); w.Code != 400 {
			t.Errorf("batch: %d", w.Code)
		}
		if w := ta.do(t, "DELETE", items, ann, bad); w.Code != 400 {
			t.Errorf("remove batch: %d", w.Code)
		}
	}
	if w := ta.do(t, "DELETE", items, ann, map[string]any{"ids": []int64{1, 2, 3}}); w.Code != 200 {
		t.Fatalf("%d", w.Code)
	}
	got = decode[map[string][]int64](t, ta.do(t, "GET", items, ann, nil))
	if len(got["ids"]) != 1096 || slices.Contains(got["ids"], 1) {
		t.Fatalf("after removing: %d", len(got["ids"]))
	}
}

func TestNewestFirstAndCovers(t *testing.T) {
	ta := newTestApp(t)
	clock := int64(1000)
	old := store.Now
	store.Now = func() int64 { clock++; return clock }
	t.Cleanup(func() { store.Now = old })
	al := ta.make(t, ann, 1, "Trip", true)
	items := fmt.Sprintf("/albums/%d/items", al.ID)
	for _, id := range []int64{5, 6, 7} {
		ta.do(t, "POST", items, ann, map[string]any{"ids": []int64{id}})
	}
	if got := decode[map[string][]int64](t, ta.do(t, "GET", items, ann, nil)); !slices.Equal(got["ids"], []int64{7, 6, 5}) {
		t.Fatalf("%v", got)
	}
	cover := func(u who_) int64 { return decode[[]Album](t, ta.do(t, "GET", "/albums?library=1", u, nil))[0].CoverID }
	if c := cover(ann); c != 7 {
		t.Fatalf("newest: %d", c)
	}
	// A chosen cover, while the person may see it.
	if w := ta.do(t, "PUT", "/albums/"+strconv.FormatInt(al.ID, 10), ann, map[string]any{"name": "Trip", "shared": true, "coverId": 5}); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if c := cover(ann); c != 5 {
		t.Fatalf("chosen: %d", c)
	}
	ta.srv.hidden[3] = map[int64]bool{5: true, 7: true}
	if c := cover(bob); c != 6 {
		t.Fatalf("bob's cover: %d", c)
	}
	// Back to the newest; and covers from elsewhere are refused.
	ta.do(t, "PUT", "/albums/"+strconv.FormatInt(al.ID, 10), ann, map[string]any{"name": "Trip", "shared": true, "coverId": 0})
	if c := cover(ann); c != 7 {
		t.Fatalf("reset: %d", c)
	}
	for _, c := range []int64{2001, 9001, 999999} {
		if w := ta.do(t, "PUT", "/albums/"+strconv.FormatInt(al.ID, 10), ann, map[string]any{"name": "Trip", "coverId": c}); w.Code != 400 {
			t.Errorf("cover %d: %d", c, w.Code)
		}
	}
	ta.srv.hidden[2] = map[int64]bool{6: true}
	if w := ta.do(t, "PUT", "/albums/"+strconv.FormatInt(al.ID, 10), ann, map[string]any{"name": "Trip", "coverId": 6}); w.Code != 400 {
		t.Errorf("a cover ann can't see: %d", w.Code)
	}
}

func TestAlbumsHoldingAnItem(t *testing.T) {
	ta := newTestApp(t)
	a1 := ta.make(t, ann, 1, "One", false)
	ta.make(t, ann, 1, "Two", false)
	ta.do(t, "POST", fmt.Sprintf("/albums/%d/items", a1.ID), ann, map[string]any{"ids": []int64{42}})
	got := decode[[]Album](t, ta.do(t, "GET", "/albums?library=1&item=42", ann, nil))
	if len(got) != 1 || got[0].ID != a1.ID {
		t.Fatalf("%+v", got)
	}
	if w := ta.do(t, "GET", "/albums?library=1&item=x", ann, nil); w.Code != 400 {
		t.Errorf("%d", w.Code)
	}
}

func TestMap(t *testing.T) {
	ta := newTestApp(t)
	get := func() MapSource { return decode[MapSource](t, ta.do(t, "GET", "/map", guest, nil)) }
	if m := get(); m != (MapSource{}) {
		t.Fatalf("default: %+v", m)
	}
	ta.srv.settings = map[string]any{"mapOnline": true, "mapTiles": "https://tiles.example/{z}/{x}/{y}.png", "mapAttribution": "© Example contributors"}
	if m := get(); m.Tiles != "https://tiles.example/{z}/{x}/{y}.png" || m.Attribution != "© Example contributors" {
		t.Fatalf("on: %+v", m)
	}
	ta.srv.settings["mapAttribution"] = "<img src=x onerror=alert(1)>"
	if m := get(); m.Attribution != "" || m.Tiles == "" {
		t.Fatalf("bad credit: %+v", m)
	}
	ta.srv.settings["mapOnline"] = false
	if m := get(); m != (MapSource{}) {
		t.Fatalf("off: %+v", m)
	}
	ta.srv.settings["mapOnline"] = true
	for _, bad := range []string{"http://tiles.example/{z}/{x}/{y}.png", "https://tiles.example/{z}/{x}.png", "https://t.example/{z}/{x}/{y}.png\"><script>", "javascript:alert(1)//{z}{x}{y}"} {
		ta.srv.settings["mapTiles"] = bad
		if m := get(); m.Tiles != "" {
			t.Errorf("%q passed", bad)
		}
	}
}

func TestValidMapTiles(t *testing.T) {
	for v, want := range map[string]bool{
		"": true, "https://a.example/{z}/{x}/{y}.png": true, "https://a.example/{z}/{y}/{x}@2x.png?key=1": true,
		"http://a.example/{z}/{x}/{y}.png": false, "https://a.example/{z}/{x}.png": false, "https://a.example/{z}/{x}/{y}.png'": false,
		"https://a.example/{z}/{x}/{y} .png": false, "https://a.example/" + strings.Repeat("a", 400) + "{z}/{x}/{y}": false,
	} {
		if ValidMapTiles(v) != want {
			t.Errorf("%q: want %v", v, want)
		}
	}
	if ValidMapCredit("a\nb") || ValidMapCredit(strings.Repeat("a", 201)) || !ValidMapCredit("© OpenStreetMap contributors") {
		t.Error("credit")
	}
}

func TestEventsForget(t *testing.T) {
	ta := newTestApp(t)
	ta.make(t, ann, 1, "Ann's", true)
	ta.make(t, bob, 1, "Bob's", true)
	ta.make(t, bob, 2, "Bob's work", true)
	ta.Heard(context.Background(), []host.Event{{Type: "user.deleted", Data: json.RawMessage(`{"user":2}`)}, {Type: "library.removed", Data: json.RawMessage(`{"library":2}`)}, {Type: "scan.finished"}})
	if got := decode[[]Album](t, ta.do(t, "GET", "/albums?library=1", admin, nil)); len(got) != 1 || got[0].Name != "Bob's" {
		t.Fatalf("%+v", got)
	}
	var n int
	_ = ta.DB.QueryRow(`SELECT COUNT(*) FROM albums WHERE library_id = 2`).Scan(&n)
	if n != 0 {
		t.Fatalf("library 2 kept %d", n)
	}
}

func TestUninstall(t *testing.T) {
	ta := newTestApp(t)
	ta.make(t, ann, 1, "a", false)
	if err := Lifecycle(context.Background(), wittymodule.CallUninstall, wittymodule.LifecycleArgs{KeepData: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ta.dir); err != nil {
		t.Fatal("kept data went")
	}
	if err := Lifecycle(context.Background(), wittymodule.CallUninstall, wittymodule.LifecycleArgs{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ta.dir); !os.IsNotExist(err) {
		t.Fatal("its folder stayed")
	}
}

func TestSources(t *testing.T) {
	ta := newTestApp(t)
	al := ta.make(t, ann, 1, "Holiday", true)
	ta.do(t, "POST", fmt.Sprintf("/albums/%d/items", al.ID), ann, map[string]any{"ids": []int64{3, 4, 5}})
	ta.srv.hidden[3] = map[int64]bool{4: true}
	bobView := sdui.Viewer{User: 3, ProfileKind: "adult"}
	ans, err := ta.Items(context.Background(), sdui.ItemsRequest{Source: sdui.Src(ui.SourceAlbums, "library", float64(1)), Viewer: bobView})
	if err != nil || len(ans.Items) != 1 {
		t.Fatalf("%v %+v", err, ans)
	}
	it := ans.Items[0]
	if it.Title != "Holiday" || it.Count != 2 || !strings.Contains(it.Subtitle, "from ann") || !strings.HasPrefix(it.Route, "/page/"+ui.PageAlbum+"?item=") || it.Images["square"] != "/api/items/5/image?w=480" {
		t.Fatalf("%+v", it)
	}
	ans, err = ta.Items(context.Background(), sdui.ItemsRequest{Source: sdui.Src(ui.SourceAlbum, "album", strconv.FormatInt(al.ID, 10)), Viewer: bobView, Limit: 1})
	if err != nil || len(ans.Items) != 1 || ans.Total != 2 || !ans.More || ans.Items[0].Kind != "photo" || ans.Items[0].Route != "/item/5" || ans.Items[0].Title != "IMG_5" {
		t.Fatalf("%v %+v", err, ans)
	}
	// Someone who may not see the album, nobody, and a library that isn't a photo library.
	ta.do(t, "PUT", fmt.Sprintf("/albums/%d", al.ID), ann, map[string]any{"name": "Holiday", "shared": false})
	if ans, _ := ta.Items(context.Background(), sdui.ItemsRequest{Source: sdui.Src(ui.SourceAlbum, "album", al.ID), Viewer: bobView}); len(ans.Items) != 0 {
		t.Fatalf("private: %+v", ans)
	}
	if ans, _ := SDUIItems(sdui.ItemsRequest{Source: sdui.Src(ui.SourceAlbums, "library", 1)}); len(ans.Items) != 0 {
		t.Fatal("nobody")
	}
	if ans, _ := ta.Items(context.Background(), sdui.ItemsRequest{Source: sdui.Src(ui.SourceAlbums, "library", 9), Viewer: bobView}); len(ans.Items) != 0 {
		t.Fatal("films")
	}
}

// Every route integration.json lists is one the service answers, and the other way round.
func TestIntegrationRoutes(t *testing.T) {
	b, err := os.ReadFile("../integration.json")
	if err != nil {
		t.Fatal(err)
	}
	in, err := wittymodule.ParseIntegration(b)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Routes(mux)
	n := 0
	for _, r := range in.HTTP {
		for _, m := range r.Methods {
			n++
			req := httptest.NewRequest(m, strings.ReplaceAll(r.Path, "{id}", "1"), nil)
			if _, pattern := mux.Handler(req); pattern == "" {
				t.Errorf("%s %s isn't answered", m, r.Path)
			}
		}
	}
	if n != 8 {
		t.Errorf("integration.json lists %d routes; the service answers 8", n)
	}
}
