export type User = {
  id: string
  email: string
  name: string
  created_at: string
}

export type Tokens = {
  access_token: string
  refresh_token: string
  access_expires_at?: string
  refresh_expires_at?: string
}

export type Project = {
  id: string
  name: string
  created_by: string
  role?: string
  created_at: string
  updated_at: string
}

export type Recording = {
  id: string
  project_id: string
  status: string
  content_type: string
  correlation_id: string
  created_at: string
  updated_at: string
}

export type Report = {
  id: string
  recording_id: string
  project_id: string
  status: string
  title: string
  summary: string
  steps: unknown
  ai_status: string
  prompt_version: string
  created_at: string
  updated_at: string
}

export type Frame = {
  ordinal: number
  content_type: string
  url?: string
}

export type ReportDetail = {
  report: Report
  recording_status: string
  metadata: unknown
  frames: Frame[]
  thumb_url?: string
}

export type Share = {
  id: string
  report_id: string
  project_id: string
  token: string
  url_path: string
  created_at: string
}

export type PublicShare = {
  title: string
  summary: string
  steps: unknown
  status: string
  frames: Frame[]
  thumb_url?: string
}

export type Comment = {
  id: string
  report_id: string
  project_id: string
  author_id: string
  author_name: string
  body: string
  created_at: string
}
