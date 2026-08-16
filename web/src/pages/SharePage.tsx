import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, ApiError } from '../api/client'
import type { PublicShare } from '../api/types'
import { StatusPill, StepsList } from '../components/ui'

export function SharePage() {
  const { token = '' } = useParams()
  const [view, setView] = useState<PublicShare | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const data = await api.publicShare(token)
        if (!cancelled) setView(data)
      } catch (err) {
        if (!cancelled) setError(err instanceof ApiError ? err.message : 'Share unavailable')
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [token])

  return (
    <div className="app-shell stack">
      <div className="brand">
        Bug<span>Sathi</span>
      </div>
      {error ? <div className="error">{error}</div> : null}
      {!view && !error ? <p className="muted">Loading shared report…</p> : null}
      {view ? (
        <>
          <div className="row space-between">
            <h1>{view.title || 'Shared report'}</h1>
            <StatusPill status={view.status} />
          </div>
          <p>{view.summary}</p>
          <section className="panel panel-pad stack">
            <h2>Reproduction steps</h2>
            <StepsList steps={view.steps} />
          </section>
          <section className="panel panel-pad stack">
            <h2>Frames</h2>
            <div className="frames">
              {view.frames.map((f) => (
                <img key={f.ordinal} src={f.url} alt={`Frame ${f.ordinal}`} />
              ))}
            </div>
          </section>
        </>
      ) : null}
      <p className="muted">
        <Link to="/login">Sign in</Link> to create your own reports.
      </p>
    </div>
  )
}
