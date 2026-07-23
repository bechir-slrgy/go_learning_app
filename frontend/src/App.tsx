import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, clearTokens, isLoggedIn } from './api/client'
import type { User } from './api/types'
import { AdminQueue } from './components/AdminQueue'
import { Login } from './components/Login'
import { Notifications } from './components/Notifications'
import { TaskList } from './components/TaskList'

type Tab = 'tasks' | 'review' | 'notifications'

export default function App() {
  const [user, setUser] = useState<User | null>(null)
  const [tab, setTab] = useState<Tab>('tasks')
  const [unread, setUnread] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const refreshUnread = useCallback(async () => {
    try {
      const notes = await api.listNotifications()
      setUnread(notes.filter((n) => !n.read).length)
    } catch {
    }
  }, [])

  const loadUser = useCallback(async () => {
    if (!isLoggedIn()) {
      setUser(null)
      setLoading(false)
      return
    }
    try {
      setUser(await api.me())
      setError('')
      void refreshUnread()
    } catch (e) {
      clearTokens()
      setUser(null)
      if (e instanceof ApiError && e.status !== 401) setError(e.message)
      else if (!(e instanceof ApiError)) setError('Cannot reach the API. Is `go run ./cmd/api` running?')
    } finally {
      setLoading(false)
    }
  }, [refreshUnread])

  useEffect(() => {
    void loadUser()
  }, [loadUser])

  function onAuthed() {
    setLoading(true)
    void loadUser()
  }

  async function logout() {
    await api.logout()
    setUser(null)
    setTab('tasks')
  }

  if (loading) return <main className="wrap"><p className="muted">Loading…</p></main>

  if (!user) {
    return (
      <main className="wrap">
        {error && <p className="error">{error}</p>}
        <Login onAuthed={onAuthed} />
      </main>
    )
  }

  const isAdmin = user.role === 'admin'

  return (
    <main className="wrap">
      <header className="header">
        <div>
          <strong>{user.name}</strong> <span className="badge">{user.role}</span>
          <div className="muted small">{user.email}</div>
        </div>
        <button onClick={() => void logout()}>Sign out</button>
      </header>

      <nav className="tabs">
        <button className={tab === 'tasks' ? 'active' : ''} onClick={() => setTab('tasks')}>
          My tasks
        </button>
        {isAdmin && (
          <button className={tab === 'review' ? 'active' : ''} onClick={() => setTab('review')}>
            Review queue
          </button>
        )}
        <button
          className={tab === 'notifications' ? 'active' : ''}
          onClick={() => setTab('notifications')}
        >
          Notifications{unread > 0 && <span className="count">{unread}</span>}
        </button>
      </nav>

      <div className="card">
        {tab === 'tasks' && <TaskList onChange={refreshUnread} />}
        {tab === 'review' && isAdmin && <AdminQueue onChange={refreshUnread} />}
        {tab === 'notifications' && <Notifications onChange={refreshUnread} />}
      </div>
    </main>
  )
}
