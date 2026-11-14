# Changelog

What changed in WittyPhotos, newest first. What changed before it had a repository of its own is in the
[WittyMediaServer changelog](https://github.com/wittywilla/WittyMediaServer/blob/master/CHANGELOG.md).

## Unreleased

- **WittyPhotos 0.5.0: module API 5, owning albums, the photo map's settings and its screens.** It is now a service (WittyModule 0.5.2's `service`): the server's scans ask it for a photo's EXIF, size and RAW preview as service calls (`read`, `dimensions`, `preview`), and it answers its own routes at `/api/modules/wittyphotos/...`, listed in `integration.json`:
  - **Albums** (`GET`/`POST /albums`, `PUT`/`DELETE /albums/{id}`, `GET`/`POST`/`DELETE /albums/{id}/items`), kept in its own SQLite database in its data folder, starting empty. The rules are the server's as they were: a name of up to 200 characters, 200 albums for an account that isn't an administrator, batches of up to 2000 pictures, Kids and Guest profiles look but don't change. What a person sees of an album, its count and its cover, are always only the pictures the server says they may see.
  - **The photo map's online map** (`GET /map`), from its own settings (Settings, Photos: whether to use one, its address of map squares, its credit), checked as the server checked them.
  - An account that goes takes its albums with it, and so does a library; removing WittyPhotos without keeping its data removes its folder.
- **Albums on every app** (`sdui.json`, written from package `ui`): an Albums row on a photo library's page and on the Photos page, a page of a library's albums and a page of an album's pictures, for the Apple TV and Android TV apps among others. The web and desktop apps' views of a library (`wittyphotos.timeline`, `wittyphotos.map`) are its bundle's.
- **Its screens in the web app** (`integration.json` `web`, `register` in `web/`): a photo library opens at `/photos/library/{id}`, drawn by its web part: the timeline, albums, favourites, folders, the map and the picture viewer, moved file for file from WittyMediaServer's web app (`PhotosLibrary`, `PhotoMap`, `PhotoViewer`, `mapMath`) with their stylesheet, built with WittyModule's kit (`npm run build:bundle`). An album's link for people without an account shares its pictures as they are when it's made (the server's share links of a set of pictures).

## 0.4.0

- **Module API 4**, for WittyMediaServer 0.8.0-alpha and later: the module carries its own server-driven UI in `sdui.json`, which `wittymodule pack` puts in every archive.
- **Its own server-driven pages** (`sdui.json`): the page of every photo library (the timeline, with albums, favourites, folders, the map and people, where the app has it; every photo in a grid on televisions and phones) and the Photos page, and a row for the page editor's gallery.
- **WittyPhotos 0.4.0:** Fits WittyMediaServer 0.8.x.

- Module API 3 and WittyMediaServer 0.6.0-alpha: this is how a server gets it now, with no copy compiled in.
- **Fixed, serious:** a 100-byte file with a Fuji RAW header recursed until the program ended, so a helper or a server reading it crashed, every time it read it. A crafted TIFF or RAW no longer takes minutes to read.
- WittyPhotos is now a project of its own, developed and released from this repository.
