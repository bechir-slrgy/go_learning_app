import { useCallback, useEffect, useState } from 'react'
import { api, ApiError } from '../api/client'
import type { Task } from '../api/types'

const PAGE_SIZE = 20

export function TaskList({ onChange }: { onChange: () => void }) {
  const [tasks, setTasks] = useState<Task[]>([])
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [title, setTitle] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async (p: number) => {
    try {
      const res = await api.listTasks(p, PAGE_SIZE)
      // A page can empty out (e.g. after deletes); fall back to the last page.
      if (res.total_pages > 0 && p > res.total_pages) {
        setPage(res.total_pages)
        return
      }
      setTasks(res.items)
      setTotal(res.total)
      setTotalPages(Math.max(1, res.total_pages))
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    void load(page)
  }, [page, load])

  async function run(fn: () => Promise<unknown>, resetToFirst = false) {
    setBusy(true)
    try {
      await fn()
      if (resetToFirst) setPage(1)
      await load(resetToFirst ? 1 : page)
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
          }, true)
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
          onClick={() => void run(() => api.importTasks(5), true)}
        >
          Import 5
        </button>
      </form>

      {error && <p className="error">{error}</p>}

      {total === 0 ? (
        <p className="muted">No tasks yet.</p>
      ) : (
        <>
          <ul className="list">
            {tasks.map((t) => (
              <TaskRow key={t.id} task={t} busy={busy} run={run} />
            ))}
          </ul>

          <div className="row pager">
            <button
              type="button"
              disabled={busy || page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              Prev
            </button>
            <span className="muted grow center">
              Page {page} of {totalPages} · {total} task{total === 1 ? '' : 's'}
            </span>
            <button
              type="button"
              disabled={busy || page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
            </button>
          </div>
        </>
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
