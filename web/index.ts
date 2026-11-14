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

// WittyPhotos' web half (module API 5): its gallery in the web app, registered by register(host),
// and the views a server-driven page names. The server half is the Go service of the same name.
export { register, views } from './app/register'
