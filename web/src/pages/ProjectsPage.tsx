import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { api, ApiError } from '../api/client'
import type { Project } from '../api/types'

export function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)

  async function load() {
    setLoading(true)
    setError('')
    try {
      const res = await api.listProjects()
      setProjects(res.projects ?? [])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to load projects')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const res = await api.createProject(name.trim())
      setName('')
      setProjects((prev) => [res.project, ...prev])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Create failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="stack">
      <div className="row space-between">
        <div>
          <h1>Projects</h1>
          <p className="muted">Workspaces for recordings, AI reports, and shared links.</p>
        </div>
      </div>

      <form className="panel panel-pad row" onSubmit={(e) => void onCreate(e)}>
        <label style={{ flex: 1, minWidth: 220 }}>
          New project
          <input
            required
            placeholder="Checkout flow"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <button className="btn btn-signal" type="submit" disabled={busy} style={{ alignSelf: 'end' }}>
          {busy ? 'Creating…' : 'Create'}
        </button>
      </form>

      {error ? <div className="error">{error}</div> : null}

      {loading ? (
        <p className="muted">Loading…</p>
      ) : projects.length === 0 ? (
        <div className="empty">No projects yet. Create one to capture your first bug.</div>
      ) : (
        <ul className="list">
          {projects.map((p) => (
            <li key={p.id}>
              <Link to={`/projects/${p.id}`}>
                <div>
                  <strong>{p.name}</strong>
                  <div className="muted mono">{p.role ?? 'member'}</div>
                </div>
                <span className="muted mono">{new Date(p.updated_at).toLocaleString()}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
