import { useState } from 'react'

const DEMO_USERS = [
  { name: 'Alice', role: 'admin', token: 'alice-token-123' },
  { name: 'Bob', role: 'member', token: 'bob-token-456' },
]

export function Login({ onLogin }: { onLogin: (token: string) => void }) {
  const [manual, setManual] = useState('')

  return (
    <div className="card login">
      <h1>Task API</h1>
      <p className="muted">
        There is no password login: the API authenticates with a bearer token.
        Pick a seeded user, or paste a token from <code>POST /api/users</code>.
      </p>

      <div className="stack">
        {DEMO_USERS.map((u) => (
          <button key={u.token} className="primary" onClick={() => onLogin(u.token)}>
            Sign in as {u.name} <span className="badge">{u.role}</span>
          </button>
        ))}
      </div>

      <form
        className="row"
        onSubmit={(e) => {
          e.preventDefault()
          if (manual.trim()) onLogin(manual.trim())
        }}
      >
        <input
          value={manual}
          onChange={(e) => setManual(e.target.value)}
          placeholder="or paste a token"
        />
        <button type="submit">Use</button>
      </form>
    </div>
  )
}
