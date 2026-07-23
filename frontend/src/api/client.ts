import type {
  Notification,
  Task,
  TaskStatus,
  TokenPair,
  User,
  Webhook,
} from './types'

const ACCESS_KEY = 'task_api_access'
const REFRESH_KEY = 'task_api_refresh'

export function getAccess(): string {
  return localStorage.getItem(ACCESS_KEY) ?? ''
}

function getRefresh(): string {
  return localStorage.getItem(REFRESH_KEY) ?? ''
}

function storeTokens(t: TokenPair): void {
  localStorage.setItem(ACCESS_KEY, t.access_token)
  localStorage.setItem(REFRESH_KEY, t.refresh_token)
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_KEY)
  localStorage.removeItem(REFRESH_KEY)
}

export function isLoggedIn(): boolean {
  return getAccess() !== ''
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

// send makes one request with the current access token. It is the low-level
// call; request() wraps it with the refresh-on-401 retry.
async function send(path: string, options: RequestInit): Promise<Response> {
  const access = getAccess()
  return fetch(`/api${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(access ? { Authorization: `Bearer ${access}` } : {}),
      ...options.headers,
    },
  })
}

// tryRefresh spends the refresh token for a new pair. Returns true on success.
// Concurrent 401s could each trigger this; a production client would
// de-duplicate with a shared promise. Kept simple here.
async function tryRefresh(): Promise<boolean> {
  const refresh = getRefresh()
  if (!refresh) return false

  const res = await fetch('/api/auth/refresh', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refresh }),
  })
  if (!res.ok) {
    clearTokens()
    return false
  }
  storeTokens((await res.json()) as TokenPair)
  return true
}

// request runs a call, and on a 401 tries exactly once to refresh the access
// token and replay it. This is why a 15-minute access token is invisible to
// the user: it silently renews behind the request.
async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  let res = await send(path, options)

  if (res.status === 401 && (await tryRefresh())) {
    res = await send(path, options)
  }

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { message?: string } | null
    throw new ApiError(res.status, body?.message ?? res.statusText)
  }
  if (res.status === 204) {
    return undefined as T
  }
  return res.json() as Promise<T>
}

export const api = {
  // --- auth (no token needed) ---
  login: async (email: string, password: string): Promise<void> => {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    })
    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as { message?: string } | null
      throw new ApiError(res.status, body?.message ?? 'login failed')
    }
    storeTokens((await res.json()) as TokenPair)
  },
  signup: (email: string, name: string, password: string) =>
    request<User>('/users', { method: 'POST', body: JSON.stringify({ email, name, password }) }),
  logout: async (): Promise<void> => {
    const refresh = getRefresh()
    clearTokens()
    if (refresh) {
      // Best-effort revoke; the local tokens are already gone.
      await fetch('/api/auth/logout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refresh }),
      }).catch(() => {})
    }
  },

  // --- users ---
  me: () => request<User>('/users/me'),
  listUsers: () => request<User[]>('/users'),

  // --- tasks (member) ---
  listTasks: () => request<Task[]>('/tasks'),
  createTask: (title: string) =>
    request<Task>('/tasks', { method: 'POST', body: JSON.stringify({ title }) }),
  updateTask: (id: number, title: string) =>
    request<Task>(`/tasks/${id}`, { method: 'PUT', body: JSON.stringify({ title }) }),
  deleteTask: (id: number) => request<void>(`/tasks/${id}`, { method: 'DELETE' }),
  submitTask: (id: number) => request<Task>(`/tasks/${id}/submit`, { method: 'POST' }),
  importTasks: (limit: number) =>
    request<Task[]>(`/tasks/import?limit=${limit}`, { method: 'POST' }),

  // --- admin only: 403 for a member ---
  reviewQueue: (status: TaskStatus) => request<Task[]>(`/admin/tasks?status=${status}`),
  approveTask: (id: number) =>
    request<Task>(`/admin/tasks/${id}/approve`, { method: 'POST' }),
  rejectTask: (id: number) =>
    request<Task>(`/admin/tasks/${id}/reject`, { method: 'POST' }),

  // --- notifications ---
  listNotifications: () => request<Notification[]>('/notifications'),
  markRead: (id: number) => request<void>(`/notifications/${id}/read`, { method: 'POST' }),

  // --- webhooks ---
  listWebhooks: () => request<Webhook[]>('/webhooks'),
  createWebhook: (url: string) =>
    request<Webhook>('/webhooks', { method: 'POST', body: JSON.stringify({ url }) }),
  deleteWebhook: (id: number) => request<void>(`/webhooks/${id}`, { method: 'DELETE' }),
}
