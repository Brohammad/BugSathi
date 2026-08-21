import { useEffect, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, ApiError } from '../api/client'
import type { Comment, ReportDetail } from '../api/types'
import { StatusPill, StepsList } from '../components/ui'

function accessToken(): string | null {
  return localStorage.getItem('bs_access')
}

export function ReportPage() {
  const { projectId = '', reportId = '' } = useParams()
  const [detail, setDetail] = useState<ReportDetail | null>(null)
  const [comments, setComments] = useState<Comment[]>([])
  const [presence, setPresence] = useState(0)
  const [body, setBody] = useState('')
  const [shareUrl, setShareUrl] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function load() {
    setError('')
    try {
      const [d, c] = await Promise.all([
        api.getReport(projectId, reportId),
        api.listComments(projectId, reportId),
      ])
      setDetail(d)
      setComments(c.comments ?? [])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to load report')
    }
  }

  useEffect(() => {
    void load()
  }, [projectId, reportId])

  useEffect(() => {
    const token = accessToken()
    if (!token || !projectId || !reportId) return

    const url =
      `/v1/projects/${projectId}/reports/${reportId}/events` +
      `?access_token=${encodeURIComponent(token)}`
    const es = new EventSource(url)

    es.addEventListener('comment.created', (ev) => {
      try {
        const comment = JSON.parse((ev as MessageEvent).data) as Comment
        setComments((prev) => (prev.some((c) => c.id === comment.id) ? prev : [...prev, comment]))
      } catch {
        /* ignore malformed */
      }
    })
    es.addEventListener('presence.updated', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { users?: unknown[] }
        if (Array.isArray(data.users)) setPresence(data.users.length)
      } catch {
        /* ignore */
      }
    })
    es.onerror = () => {
      // Browser auto-reconnects; surface nothing sticky for transient blips.
    }

    return () => es.close()
  }, [projectId, reportId])

  async function onShare() {
    setBusy(true)
    setError('')
    try {
      const res = await api.createShare(projectId, reportId)
      const url = `${window.location.origin}/share/${res.share.token}`
      setShareUrl(url)
      await navigator.clipboard.writeText(url).catch(() => undefined)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Share failed')
    } finally {
      setBusy(false)
    }
  }

  async function onComment(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const res = await api.createComment(projectId, reportId, body.trim())
      setComments((prev) => (prev.some((c) => c.id === res.comment.id) ? prev : [...prev, res.comment]))
      setBody('')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Comment failed')
    } finally {
      setBusy(false)
    }
  }

  async function onReprocess() {
    if (!detail) return
    setBusy(true)
    setError('')
    try {
      await api.reprocessRecording(projectId, detail.report.recording_id)
      setError('')
      alert('Reprocess accepted — refresh in a few seconds.')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Reprocess failed')
    } finally {
      setBusy(false)
    }
  }

  if (!detail && !error) return <p className="muted">Loading report…</p>
  if (!detail) return <div className="error">{error}</div>

  const { report, frames } = detail

  return (
    <div className="stack">
      <div>
        <p className="muted mono">
          <Link to={`/projects/${projectId}`}>← Project</Link>
        </p>
        <div className="row space-between">
          <h1>{report.title || 'Report'}</h1>
          <StatusPill status={report.status} />
        </div>
        <p>{report.summary}</p>
        {presence > 0 ? <p className="muted">{presence} viewing</p> : null}
      </div>

      {error ? <div className="error">{error}</div> : null}

      <div className="row">
        <button type="button" className="btn btn-signal" disabled={busy} onClick={() => void onShare()}>
          Create share link
        </button>
        <button type="button" className="btn btn-ghost" disabled={busy} onClick={() => void onReprocess()}>
          Reprocess recording
        </button>
        {shareUrl ? (
          <a className="mono" href={shareUrl} target="_blank" rel="noreferrer">
            {shareUrl}
          </a>
        ) : null}
      </div>

      <section className="panel panel-pad stack">
        <h2>Reproduction steps</h2>
        <StepsList steps={report.steps} />
      </section>

      <section className="panel panel-pad stack">
        <h2>Frames</h2>
        {frames.length === 0 ? (
          <p className="muted">No frames yet.</p>
        ) : (
          <div className="frames">
            {frames.map((f) => (
              <a key={f.ordinal} href={f.url} target="_blank" rel="noreferrer">
                <img src={f.url} alt={`Frame ${f.ordinal}`} />
              </a>
            ))}
          </div>
        )}
      </section>

      <section className="panel panel-pad stack">
        <h2>Comments</h2>
        <div className="comments">
          {comments.length === 0 ? <p className="muted">No comments yet.</p> : null}
          {comments.map((c) => (
            <div className="comment" key={c.id}>
              <div className="meta">
                {c.author_name} · {new Date(c.created_at).toLocaleString()}
              </div>
              <div>{c.body}</div>
            </div>
          ))}
        </div>
        <form className="stack" onSubmit={(e) => void onComment(e)}>
          <label>
            Add a comment
            <textarea rows={3} required value={body} onChange={(e) => setBody(e.target.value)} />
          </label>
          <button className="btn" type="submit" disabled={busy}>
            Post
          </button>
        </form>
      </section>
    </div>
  )
}
