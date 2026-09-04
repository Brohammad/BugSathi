import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, ApiError } from '../api/client'
import { StatusPill } from '../components/ui'
import { RECORDING_ACCEPT, recordingContentType } from '../media/contentType'

type Phase = 'idle' | 'recording' | 'uploading' | 'processing' | 'done' | 'error'

export function RecordPage() {
  const { projectId = '' } = useParams()
  const nav = useNavigate()
  const videoRef = useRef<HTMLVideoElement>(null)
  const recorderRef = useRef<MediaRecorder | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const streamRef = useRef<MediaStream | null>(null)

  const [phase, setPhase] = useState<Phase>('idle')
  const [status, setStatus] = useState('')
  const [error, setError] = useState('')
  const [recordingId, setRecordingId] = useState('')
  const [file, setFile] = useState<File | null>(null)

  useEffect(() => {
    return () => {
      streamRef.current?.getTracks().forEach((t) => t.stop())
    }
  }, [])

  async function startCapture() {
    setError('')
    try {
      const stream = await navigator.mediaDevices.getDisplayMedia({
        video: true,
        audio: true,
      })
      streamRef.current = stream
      if (videoRef.current) {
        videoRef.current.srcObject = stream
        await videoRef.current.play()
      }
      chunksRef.current = []
      const mime = MediaRecorder.isTypeSupported('video/webm;codecs=vp9,opus')
        ? 'video/webm;codecs=vp9,opus'
        : 'video/webm'
      const recorder = new MediaRecorder(stream, { mimeType: mime })
      recorder.ondataavailable = (ev) => {
        if (ev.data.size > 0) chunksRef.current.push(ev.data)
      }
      recorder.onstop = () => {
        void uploadBlob(new Blob(chunksRef.current, { type: 'video/webm' }), 'bug-capture.webm')
      }
      recorderRef.current = recorder
      recorder.start(1000)
      setPhase('recording')
      stream.getVideoTracks()[0]?.addEventListener('ended', () => {
        if (recorderRef.current?.state === 'recording') recorderRef.current.stop()
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not start screen capture')
      setPhase('error')
    }
  }

  function stopCapture() {
    recorderRef.current?.stop()
    streamRef.current?.getTracks().forEach((t) => t.stop())
    streamRef.current = null
    if (videoRef.current) videoRef.current.srcObject = null
  }

  async function uploadBlob(blob: Blob, filename: string) {
    const contentType = recordingContentType({ type: blob.type, name: filename })
    if (!contentType) {
      setError('Upload a WebM, MP4, or MOV file.')
      setPhase('error')
      return
    }
    setPhase('uploading')
    setStatus('Creating recording…')
    try {
      const created = await api.createRecording(projectId, contentType, filename, {
        browser: navigator.userAgent.includes('Chrome') ? 'chrome' : 'browser',
        os: navigator.platform || 'unknown',
        source: 'web-ui',
      })
      setRecordingId(created.recording.id)
      setStatus('Uploading to object storage…')
      await api.uploadBlob(created.upload_url, blob, contentType)
      setStatus('Completing upload…')
      await api.completeRecording(projectId, created.recording.id)
      setPhase('processing')
      setStatus('Waiting for media + AI pipeline…')
      await waitForReport(created.recording.id)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Upload failed')
      setPhase('error')
    }
  }

  async function waitForReport(rid: string) {
    for (let i = 0; i < 90; i++) {
      try {
        const rec = await api.getRecording(projectId, rid)
        setStatus(`Recording ${rec.recording.status}`)
        if (rec.recording.status === 'FAILED') {
          setError('Pipeline failed — try reprocess from the report page after fixing media/AI.')
          setPhase('error')
          return
        }
      } catch {
        /* ignore transient */
      }
      try {
        const detail = await api.getReportByRecording(projectId, rid)
        if (detail.report.status === 'READY') {
          setPhase('done')
          nav(`/projects/${projectId}/reports/${detail.report.id}`)
          return
        }
      } catch {
        /* 404 until ready */
      }
      await new Promise((r) => setTimeout(r, 2000))
    }
    setError('Timed out waiting for READY report')
    setPhase('error')
  }

  async function onFileUpload() {
    if (!file) return
    setError('')
    await uploadBlob(file, file.name || 'bug.webm')
  }

  return (
    <div className="stack">
      <div>
        <p className="muted mono">
          <Link to={`/projects/${projectId}`}>← Project</Link>
        </p>
        <h1>Capture bug</h1>
        <p className="muted">Record your screen or upload a WebM, MP4, or MOV. The worker extracts frames and generates the report.</p>
      </div>

      <section className="panel panel-pad recorder">
        <video ref={videoRef} muted playsInline />
        <div className="row">
          {phase === 'recording' ? (
            <>
              <span className="pulse" aria-hidden />
              <strong>Recording…</strong>
              <button type="button" className="btn btn-danger" onClick={stopCapture}>
                Stop & upload
              </button>
            </>
          ) : (
            <button
              type="button"
              className="btn btn-signal"
              onClick={() => void startCapture()}
              disabled={phase === 'uploading' || phase === 'processing'}
            >
              Start screen capture
            </button>
          )}
          {(phase === 'uploading' || phase === 'processing') && <StatusPill status={phase} />}
        </div>
        {status ? <p className="mono muted">{status}</p> : null}
        {recordingId ? <p className="mono muted">recording_id={recordingId}</p> : null}
        {error ? <div className="error">{error}</div> : null}
      </section>

      <section className="panel panel-pad stack">
        <h2>Or upload a file</h2>
        <input
          type="file"
          accept={RECORDING_ACCEPT}
          onChange={(e) => setFile(e.target.files?.[0] ?? null)}
        />
        <button
          type="button"
          className="btn"
          disabled={!file || phase === 'uploading' || phase === 'processing' || phase === 'recording'}
          onClick={() => void onFileUpload()}
        >
          Upload file
        </button>
      </section>
    </div>
  )
}
