import { useEffect, useState } from 'react'
import { api, ApiError } from '../api/client'
import type { Task, TaskStatus } from '../api/types'

const STATUSES: TaskStatus[] = ['submitted', 'pending', 'approved', 'rejected']

export function AdminQueue({ onChange }: { onChange: () => void }) {
  const [status, setStatus] = useState<TaskStatus>('submitted')
  const [tasks, setTasks] = useState<Task[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function load(s: TaskStatus) {
    try {
      setTasks(await api.reviewQueue(s))
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e))
    }
  }

  useEffect(() => {
    void load(status)
  }, [status])

  async function decide(id: number, approve: boolean) {
    setBusy(true)
    try {
      await (approve ? api.approveTask(id) : api.rejectTask(id))
      await load(status)
      onChange()
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section>
      <div className="row">
        <label htmlFor="status">Showing</label>
        <select
          id="status"
          value={status}
          onChange={(e) => setStatus(e.target.value as TaskStatus)}
        >
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </div>

      {error && <p className="error">{error}</p>}

      {tasks.length === 0 ? (
        <p className="muted">Nothing {status}.</p>
      ) : (
        <ul className="list">
          {tasks.map((t) => (
            <li key={t.id} className="list-item">
              <span className="grow">{t.title}</span>
              <span className="muted">user {t.user_id}</span>
              <span className={`badge status-${t.status}`}>{t.status}</span>

              {t.status === 'submitted' && (
                <>
                  <button
                    className="primary"
                    disabled={busy}
                    onClick={() => void decide(t.id, true)}
                  >
                    Approve
                  </button>
                  <button
                    className="danger"
                    disabled={busy}
                    onClick={() => void decide(t.id, false)}
                  >
                    Reject
                  </button>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
