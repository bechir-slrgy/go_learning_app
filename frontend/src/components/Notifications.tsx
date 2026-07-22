import { useEffect, useState } from 'react'
import { api, ApiError } from '../api/client'
import type { Notification } from '../api/types'

export function Notifications({ onChange }: { onChange: () => void }) {
  const [notes, setNotes] = useState<Notification[]>([])
  const [error, setError] = useState('')

  async function load() {
    try {
      setNotes(await api.listNotifications())
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e))
    }
  }

  useEffect(() => {
    void load()
  }, [])

  async function markRead(id: number) {
    try {
      await api.markRead(id)
      await load()
      onChange()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e))
    }
  }

  return (
    <section>
      {error && <p className="error">{error}</p>}

      {notes.length === 0 ? (
        <p className="muted">Nothing yet. Submit a task as Bob, then look here as Alice.</p>
      ) : (
        <ul className="list">
          {notes.map((n) => (
            <li key={n.id} className={`list-item ${n.read ? 'read' : ''}`}>
              {!n.read && <span className="dot" aria-label="unread" />}
              <span className="grow">{n.message}</span>
              <span className="muted">task {n.task_id}</span>
              {!n.read && <button onClick={() => void markRead(n.id)}>Mark read</button>}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
