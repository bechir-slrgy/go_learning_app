import { useEffect, useState } from 'react'
import { api, ApiError } from '../api/client'
import type { Task } from '../api/types'

export function TaskList({ onChange }: { onChange: () => void }) {
  const [tasks, setTasks] = useState<Task[]>([])
  const [title, setTitle] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function load() {
    try {
      setTasks(await api.listTasks())
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e))
    }
  }

  useEffect(() => {
    void load()
  }, [])

  async function run(fn: () => Promise<unknown>) {
    setBusy(true)
    try {
      await fn()
      await load()
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
      <form
        className="row"
        onSubmit={(e) => {
          e.preventDefault()
          if (!title.trim()) return
          void run(async () => {
            await api.createTask(title)
            setTitle('')
          })
        }}
      >
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="New task title"
        />
        <button className="primary" type="submit" disabled={busy}>
          Add
        </button>
        <button
          type="button"
          disabled={busy}
          title="Fetch todos from jsonplaceholder.typicode.com"
          onClick={() => void run(() => api.importTasks(5))}
        >
          Import 5
        </button>
      </form>

      {error && <p className="error">{error}</p>}

      {tasks.length === 0 ? (
        <p className="muted">No tasks yet.</p>
      ) : (
        <ul className="list">
          {tasks.map((t) => (
            <TaskRow key={t.id} task={t} busy={busy} run={run} />
          ))}
        </ul>
      )}
    </section>
  )
}

function TaskRow({
  task,
  busy,
  run,
}: {
  task: Task
  busy: boolean
  run: (fn: () => Promise<unknown>) => Promise<void>
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(task.title)

  const canSubmit = task.status === 'pending' || task.status === 'rejected'

  return (
    <li className="list-item">
      {editing ? (
        <form
          className="row grow"
          onSubmit={(e) => {
            e.preventDefault()
            void run(async () => {
              await api.updateTask(task.id, draft)
              setEditing(false)
            })
          }}
        >
          <input value={draft} onChange={(e) => setDraft(e.target.value)} autoFocus />
          <button type="submit" disabled={busy}>
            Save
          </button>
          <button type="button" onClick={() => setEditing(false)}>
            Cancel
          </button>
        </form>
      ) : (
        <>
          <span className="grow">
            {task.title}
            {task.reviewed_at && (
              <span className="muted small block">
                reviewed{task.reviewed_by ? ` by user ${task.reviewed_by}` : ''} on{' '}
                {new Date(task.reviewed_at).toLocaleString()}
              </span>
            )}
          </span>
          <span className={`badge status-${task.status}`}>{task.status}</span>

          {canSubmit && (
            <button
              className="primary"
              disabled={busy}
              onClick={() => void run(() => api.submitTask(task.id))}
            >
              Submit
            </button>
          )}
          <button disabled={busy} onClick={() => setEditing(true)}>
            Edit
          </button>
          <button
            className="danger"
            disabled={busy}
            onClick={() => void run(() => api.deleteTask(task.id))}
          >
            Delete
          </button>
        </>
      )}
    </li>
  )
}
