import { useState } from 'react'
import { api, ApiError } from '../api/client'

export function Login({ onAuthed }: { onAuthed: () => void }) {
  const [mode, setMode] = useState<'login' | 'signup'>('login')
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      if (mode === 'signup') {
        await api.signup(email, name, password)
      }
      await api.login(email, password)
      onAuthed()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="card login">
      <h1>Task API</h1>

      <div className="tabs">
        <button className={mode === 'login' ? 'active' : ''} onClick={() => setMode('login')}>
          Sign in
        </button>
        <button className={mode === 'signup' ? 'active' : ''} onClick={() => setMode('signup')}>
          Sign up
        </button>
      </div>

      <form className="stack" onSubmit={submit}>
        <input
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="Email"
          autoComplete="email"
          required
        />
        {mode === 'signup' && (
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Name"
            required
          />
        )}
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="Password"
          autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
          required
        />
        <button className="primary" type="submit" disabled={busy}>
          {mode === 'login' ? 'Sign in' : 'Create account'}
        </button>
      </form>

      {error && <p className="error">{error}</p>}

      <p className="muted small">
        Seeded users: <code>alice@example.com</code> (admin) and{' '}
        <code>bob@example.com</code> (member), password <code>password123</code>.
      </p>
    </div>
  )
}
