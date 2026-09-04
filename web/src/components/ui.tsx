import { Link, NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '../auth/auth'

export function AppShell() {
  const { user, logout } = useAuth()

  return (
    <div className="app-shell">
      <header className="topbar">
        <Link to="/" className="brand">
          Bug<span>Sathi</span>
        </Link>
        <div className="row">
          <span className="muted mono">{user?.email}</span>
          <NavLink className="btn btn-ghost" to="/">
            Projects
          </NavLink>
          <button type="button" className="btn btn-ghost" onClick={() => void logout()}>
            Log out
          </button>
        </div>
      </header>
      <Outlet />
    </div>
  )
}

export function StatusPill({ status }: { status: string }) {
  const s = status.toUpperCase()
  let cls = 'pill'
  if (s === 'READY' || s === 'UPLOADED') cls += ' ok'
  else if (s === 'FAILED') cls += ' bad'
  else if (s === 'PROCESSING' || s === 'UPLOADING' || s === 'PENDING') cls += ' warn'
  return <span className={cls}>{s}</span>
}

export function StepsList({ steps }: { steps: unknown }) {
  const items = normalizeSteps(steps)
  if (items.length === 0) return <p className="muted">No reproduction steps.</p>
  return (
    <ol className="steps">
      {items.map((step, i) => (
        <li key={`${i}-${step}`}>{step}</li>
      ))}
    </ol>
  )
}

function normalizeSteps(steps: unknown): string[] {
  if (Array.isArray(steps)) {
    return steps.map((s) => (typeof s === 'string' ? s : JSON.stringify(s)))
  }
  if (typeof steps === 'string') {
    try {
      return normalizeSteps(JSON.parse(steps))
    } catch {
      return steps ? [steps] : []
    }
  }
  return []
}
