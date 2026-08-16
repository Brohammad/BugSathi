import type {
  Comment,
  Project,
  PublicShare,
  Recording,
  Report,
  ReportDetail,
  Share,
  Tokens,
  User,
} from './types'

const API_BASE = (import.meta.env.VITE_API_URL as string | undefined)?.replace(/\/$/, '') ?? ''

export class ApiError extends Error {
  status: number
  body: string

  constructor(status: number, body: string) {
    super(body || `HTTP ${status}`)
    this.status = status
    this.body = body
  }
}

type TokenStore = {
  getAccess: () => string | null
  getRefresh: () => string | null
  setTokens: (tokens: Tokens) => void
  clear: () => void
}

let store: TokenStore = {
  getAccess: () => localStorage.getItem('bs_access'),
  getRefresh: () => localStorage.getItem('bs_refresh'),
  setTokens: (t) => {
    localStorage.setItem('bs_access', t.access_token)
    localStorage.setItem('bs_refresh', t.refresh_token)
  },
  clear: () => {
    localStorage.removeItem('bs_access')
    localStorage.removeItem('bs_refresh')
  },
}

export function configureAuthStore(next: TokenStore) {
  store = next
}

let refreshInflight: Promise<boolean> | null = null

async function refreshAccess(): Promise<boolean> {
  if (refreshInflight) return refreshInflight
  refreshInflight = (async () => {
    const refresh = store.getRefresh()
    if (!refresh) return false
    const res = await fetch(`${API_BASE}/v1/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refresh }),
    })
    if (!res.ok) {
      store.clear()
      return false
    }
    const data = (await res.json()) as { tokens: Tokens }
    store.setTokens(data.tokens)
    return true
  })().finally(() => {
    refreshInflight = null
  })
  return refreshInflight
}

async function request<T>(
  method: string,
  path: string,
  opts: { body?: unknown; auth?: boolean; retry?: boolean } = {},
): Promise<T> {
  const headers: Record<string, string> = {}
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json'
  if (opts.auth !== false) {
    const access = store.getAccess()
    if (access) headers.Authorization = `Bearer ${access}`
  }

  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })

  if (res.status === 401 && opts.auth !== false && opts.retry !== false) {
    const ok = await refreshAccess()
    if (ok) return request<T>(method, path, { ...opts, retry: false })
  }

  if (!res.ok) {
    const text = await res.text()
    let msg = text
    try {
      const j = JSON.parse(text) as { error?: string }
      if (j.error) msg = j.error
    } catch {
      /* keep raw */
    }
    throw new ApiError(res.status, msg)
  }

  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  register(email: string, password: string, name: string) {
    return request<{ user: User; tokens: Tokens }>('POST', '/v1/auth/register', {
      body: { email, password, name },
      auth: false,
    })
  },
  login(email: string, password: string) {
    return request<{ user: User; tokens: Tokens }>('POST', '/v1/auth/login', {
      body: { email, password },
      auth: false,
    })
  },
  me() {
    return request<{ user: User }>('GET', '/v1/auth/me')
  },
  logout(refreshToken: string) {
    return request<void>('POST', '/v1/auth/logout', {
      body: { refresh_token: refreshToken },
      auth: false,
    }).catch(() => undefined)
  },
  listProjects() {
    return request<{ projects: Project[] }>('GET', '/v1/projects')
  },
  createProject(name: string) {
    return request<{ project: Project }>('POST', '/v1/projects', { body: { name } })
  },
  getProject(id: string) {
    return request<{ project: Project }>('GET', `/v1/projects/${id}`)
  },
  listReports(projectId: string) {
    return request<{ reports: Report[] }>('GET', `/v1/projects/${projectId}/reports`)
  },
  getReport(projectId: string, reportId: string) {
    return request<ReportDetail>('GET', `/v1/projects/${projectId}/reports/${reportId}`)
  },
  getReportByRecording(projectId: string, recordingId: string) {
    return request<ReportDetail>('GET', `/v1/projects/${projectId}/recordings/${recordingId}/report`)
  },
  getRecording(projectId: string, recordingId: string) {
    return request<{ recording: Recording }>('GET', `/v1/projects/${projectId}/recordings/${recordingId}`)
  },
  createRecording(projectId: string, contentType: string, filename: string, metadata: Record<string, string>) {
    return request<{ recording: Recording; upload_url: string }>('POST', `/v1/projects/${projectId}/recordings`, {
      body: { content_type: contentType, filename, metadata },
    })
  },
  completeRecording(projectId: string, recordingId: string) {
    return request<{ recording: Recording }>('POST', `/v1/projects/${projectId}/recordings/${recordingId}/complete`)
  },
  reprocessRecording(projectId: string, recordingId: string) {
    return request<{ recording: Recording }>('POST', `/v1/projects/${projectId}/recordings/${recordingId}/reprocess`)
  },
  async uploadBlob(uploadUrl: string, blob: Blob, contentType: string) {
    const res = await fetch(uploadUrl, {
      method: 'PUT',
      headers: { 'Content-Type': contentType },
      body: blob,
    })
    if (!res.ok) throw new ApiError(res.status, `upload failed (${res.status})`)
  },
  createShare(projectId: string, reportId: string) {
    return request<{ share: Share }>('POST', `/v1/projects/${projectId}/reports/${reportId}/shares`, {
      body: {},
    })
  },
  publicShare(token: string) {
    return request<PublicShare>('GET', `/s/${token}`, { auth: false })
  },
  listComments(projectId: string, reportId: string) {
    return request<{ comments: Comment[] }>('GET', `/v1/projects/${projectId}/reports/${reportId}/comments`)
  },
  createComment(projectId: string, reportId: string, body: string) {
    return request<{ comment: Comment }>('POST', `/v1/projects/${projectId}/reports/${reportId}/comments`, {
      body: { body },
    })
  },
}
