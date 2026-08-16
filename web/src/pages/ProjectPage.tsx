import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, ApiError } from '../api/client'
import type { Project, Report } from '../api/types'
import { StatusPill } from '../components/ui'

export function ProjectPage() {
  const { projectId = '' } = useParams()
  const [project, setProject] = useState<Project | null>(null)
  const [reports, setReports] = useState<Report[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      setError('')
      try {
        const [p, r] = await Promise.all([api.getProject(projectId), api.listReports(projectId)])
        if (cancelled) return
        setProject(p.project)
        setReports(r.reports ?? [])
      } catch (err) {
        if (!cancelled) setError(err instanceof ApiError ? err.message : 'Failed to load project')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [projectId])

  return (
    <div className="stack">
      <div className="row space-between">
        <div>
          <p className="muted mono">
            <Link to="/">Projects</Link> / {project?.name ?? '…'}
          </p>
          <h1>{project?.name ?? 'Project'}</h1>
        </div>
        <Link className="btn btn-signal" to={`/projects/${projectId}/record`}>
          Capture bug
        </Link>
      </div>

      {error ? <div className="error">{error}</div> : null}

      <section className="panel panel-pad stack">
        <div className="row space-between">
          <h2>Reports</h2>
          <button type="button" className="btn btn-ghost" onClick={() => window.location.reload()}>
            Refresh
          </button>
        </div>
        {loading ? (
          <p className="muted">Loading…</p>
        ) : reports.length === 0 ? (
          <div className="empty">No reports yet. Capture a screen recording to kick off the pipeline.</div>
        ) : (
          <ul className="list">
            {reports.map((r) => (
              <li key={r.id}>
                <Link to={`/projects/${projectId}/reports/${r.id}`}>
                  <div>
                    <strong>{r.title || 'Untitled report'}</strong>
                    <div className="muted">{r.summary?.slice(0, 120)}</div>
                  </div>
                  <StatusPill status={r.status} />
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
