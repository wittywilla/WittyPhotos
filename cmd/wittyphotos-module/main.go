// SPDX-License-Identifier: AGPL-3.0-or-later
//
// WittyPhotos - Photo libraries: what a photo says about itself, including RAW files, without decoding it.
// Copyright (C) 2026 WittyWilla and contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Command wittyphotos-module is WittyPhotos' service (module API 5): it reads what photos say about
// themselves for the server's scans (read, dimensions, preview), answers WittyPhotos' own routes,
// which the server passes on at /api/modules/wittyphotos/... (albums, the map), the sources of its
// SDUI pages, and its lifecycle. It listens on this machine only. See package service in WittyModule.
package main

import (
	"net/http"

	"github.com/wittywilla/wittymodule/sdui"
	"github.com/wittywilla/wittymodule/service"
	"github.com/wittywilla/wittyphotos"
	"github.com/wittywilla/wittyphotos/api"
)

func main() {
	service.Main(service.Config{
		Manifest:  wittyphotos.ManifestJSON,
		Handlers:  api.Handlers,
		ErrorCode: api.ErrorCode,
		UI:        sdui.Answers{Items: api.SDUIItems},
		HTTP:      func(mux *http.ServeMux) { api.Routes(mux); api.Watch() },
		Lifecycle: api.Lifecycle,
	})
}
