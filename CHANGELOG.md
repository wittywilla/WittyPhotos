# Changelog

What changed in WittyPhotos, newest first. What changed before it had a repository of its own is in the
[WittyMediaServer changelog](https://github.com/wittywilla/WittyMediaServer/blob/master/CHANGELOG.md).

## Unreleased

- **Fixed, serious:** a 100-byte file with a Fuji RAW header recursed until the program ended, so a helper or a server reading it crashed, every time it read it. A crafted TIFF or RAW no longer takes minutes to read.
- WittyPhotos is now a project of its own, developed and released from this repository.
