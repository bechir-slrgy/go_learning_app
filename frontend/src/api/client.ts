import type {
  ApiErrorBody,
  Notification,
  Task,
  TaskStatus,
  User,
  Webhook,
} from './types'

const TOKEN_KEY = 'task_api_token'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? ''
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
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

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken()

  const res = await fetch(`/api${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  })

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ApiErrorBody | null
    throw new ApiError(res.status, body?.message ?? res.statusText)
  }

  if (res.status === 204) {
    return undefined as T
  }
  return res.json() as Promise<T>
}

export const api = {
  me: () => request<User>('/users/me'),
  listUsers: () => request<User[]>('/users'),

  listTasks: () => request<Task[]>('/tasks'),
  createTask: (title: string) =>
    request<Task>('/tasks', { method: 'POST', body: JSON.stringify({ title }) }),
  updateTask: (id: number, title: string) =>
    request<Task>(`/tasks/${id}`, { method: 'PUT', body: JSON.stringify({ title }) }),
  deleteTask: (id: number) => request<void>(`/tasks/${id}`, { method: 'DELETE' }),
  submitTask: (id: number) => request<Task>(`/tasks/${id}/submit`, { method: 'POST' }),
  importTasks: (limit: number) =>
    request<Task[]>(`/tasks/import?limit=${limit}`, { method: 'POST' }),

  reviewQueue: (status: TaskStatus) => request<Task[]>(`/admin/tasks?status=${status}`),
  approveTask: (id: number) =>
    request<Task>(`/admin/tasks/${id}/approve`, { method: 'POST' }),
  rejectTask: (id: number) =>
    request<Task>(`/admin/tasks/${id}/reject`, { method: 'POST' }),

  listNotifications: () => request<Notification[]>('/notifications'),
  markRead: (id: number) => request<void>(`/notifications/${id}/read`, { method: 'POST' }),

  listWebhooks: () => request<Webhook[]>('/webhooks'),
  createWebhook: (url: string) =>
    request<Webhook>('/webhooks', { method: 'POST', body: JSON.stringify({ url }) }),
  deleteWebhook: (id: number) => request<void>(`/webhooks/${id}`, { method: 'DELETE' }),
}
