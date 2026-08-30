import { Navigate, Outlet, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { useAuth } from './auth/auth'
import { AppShell } from './components/ui'
import { LoginPage, RegisterPage } from './pages/AuthPages'
import { ProjectPage } from './pages/ProjectPage'
import { ProjectsPage } from './pages/ProjectsPage'
import { RecordPage } from './pages/RecordPage'
import { ReportPage } from './pages/ReportPage'
import { SharePage } from './pages/SharePage'

function RequireAuth() {
  const { user, loading } = useAuth()
  if (loading) return <div className="auth-layout muted">Loading…</div>
  if (!user) return <Navigate to="/login" replace />
  return <Outlet />
}

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        {/* /s/:token is the API's public JSON endpoint, so the UI renders shares here. */}
        <Route path="/share/:token" element={<SharePage />} />
        <Route element={<RequireAuth />}>
          <Route element={<AppShell />}>
            <Route index element={<ProjectsPage />} />
            <Route path="/projects/:projectId" element={<ProjectPage />} />
            <Route path="/projects/:projectId/record" element={<RecordPage />} />
            <Route path="/projects/:projectId/reports/:reportId" element={<ReportPage />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AuthProvider>
  )
}
