// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
//
// WittyPhotos' screens in WittyMediaServer's web app (module API 5; integration.json says what it
// claims). A photos library is taken over to /photos/library/<id>, WittyPhotos' gallery: the
// timeline, albums, favourites, folders and the map, with the picture viewer. A server-driven page
// asking for the photo timeline or map (the NativeViews photoTimeline and photoMap) gets the same
// gallery through the views wittyphotos.timeline and wittyphotos.map. Every other app draws the
// SDUI pages in sdui.json instead.
import { lazy, Suspense, useEffect, type ReactNode } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { EmptyState, Spinner } from '@wms/app/components/common'
import { useApp } from '@wms/app/state'

const PhotosLibrary = lazy(() => import('./PhotosLibrary'))

const wait = (el: ReactNode) => <Suspense fallback={<Spinner />}>{el}</Suspense>

// The gallery of one library, by its number, if this profile can open it.
function Gallery({ id, map }: { id: string | undefined; map?: boolean }) {
  const { libraries } = useApp()
  const [params, setParams] = useSearchParams()
  const lib = libraries.find((l) => String(l.id) === id && l.type === 'photos')
  // The map is one of the gallery's own tabs: a page asking for it opens there, once.
  useEffect(() => {
    if (map && !params.get('tab')) { const next = new URLSearchParams(params); next.set('tab', 'map'); setParams(next, { replace: true }) }
  }, [map]) // eslint-disable-line react-hooks/exhaustive-deps
  if (!lib) return <EmptyState title="Library not found" action={<Link to="/" className="btn">Back to home</Link>} />
  return wait(<PhotosLibrary key={lib.id} lib={lib} />)
}

function LibraryRoute() {
  const { id } = useParams()
  return <Gallery id={id} />
}

type ViewProps = { library?: string }

/** The views sdui.json declares, for a server-driven page on the web and desktop apps. */
export const views = {
  'wittyphotos.timeline': ({ library }: ViewProps) => <Gallery id={library} />,
  'wittyphotos.map': ({ library }: ViewProps) => <Gallery id={library} map />,
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function register(_host: unknown): any {
  return {
    routes: {
      '/photos/library/:id': LibraryRoute,
    },
  }
}
