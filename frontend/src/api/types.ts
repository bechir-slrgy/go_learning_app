export type Role = 'admin' | 'member'

export type TaskStatus = 'pending' | 'submitted' | 'approved' | 'rejected'

// IDs are UUID strings, matching the Go uuid.UUID fields which marshal to a
// JSON string. They were numbers before the int -> UUID migration.
export interface User {
  id: string
  email: string
  name: string
  role: Role
  created_at: string
}

export interface Task {
  id: string
  user_id: string
  title: string
  status: TaskStatus
  reviewed_by: string | null
  reviewed_at: string | null
  created_at: string
}

export interface Notification {
  id: string
  user_id: string
  task_id: string
  message: string
  read: boolean
  created_at: string
}

export interface Webhook {
  id: string
  user_id: string
  url: string
  created_at: string
}

export interface ApiErrorBody {
  status: number
  message: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: string
  access_expires_in: number
}
