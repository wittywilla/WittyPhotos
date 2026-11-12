# Changelog

What changed in WittyPhotos, newest first. What changed before it had a repository of its own is in the
[WittyMediaServer changelog](https://github.com/wittywilla/WittyMediaServer/blob/master/CHANGELOG.md).

## Unreleased

- **Its own server-driven pages** (`sdui.json`): the page of every photo library (the timeline, with albums, favourites, folders, the map and people, where the app has it; every photo in a grid on televisions and phones) and the Photos page, and a row for the page editor's gallery.
- **WittyPhotos 0.4.0:** Fits WittyMediaServer 0.8.x.

- Module API 3 and WittyMediaServer 0.6.0-alpha: this is how a server gets it now, with no copy compiled in.
- **Fixed, serious:** a 100-byte file with a Fuji RAW header recursed until the program ended, so a helper or a server reading it crashed, every time it read it. A crafted TIFF or RAW no longer takes minutes to read.
- WittyPhotos is now a project of its own, developed and released from this repository.
