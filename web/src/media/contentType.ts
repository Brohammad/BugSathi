const ALLOWED = new Set(['video/webm', 'video/mp4', 'video/quicktime'])

const EXT_TO_TYPE: Record<string, string> = {
  webm: 'video/webm',
  mp4: 'video/mp4',
  m4v: 'video/mp4',
  mov: 'video/quicktime',
}

function normalizeType(ct: string): string {
  const raw = ct.toLowerCase().trim()
  const semi = raw.indexOf(';')
  return (semi >= 0 ? raw.slice(0, semi) : raw).trim()
}

/** MIME types the upload API will accept. */
export const RECORDING_ACCEPT = 'video/webm,video/mp4,video/quicktime,.webm,.mp4,.m4v,.mov'

/**
 * Resolve a recording content type from the file's MIME and name.
 * Screen capture should pass type `video/webm`.
 */
export function recordingContentType(file: { type: string; name: string }): string | null {
  const fromType = normalizeType(file.type)
  if (ALLOWED.has(fromType)) return fromType
  const ext = file.name.split('.').pop()?.toLowerCase() ?? ''
  return EXT_TO_TYPE[ext] ?? null
}
