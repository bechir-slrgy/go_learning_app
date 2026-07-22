export type Role = 'admin' | 'member'

export type TaskStatus = 'pending' | 'submitted' | 'approved' | 'rejected'

export interface User {
  id: number
  email: string
  name: string
  role: Role
  created_at: string
}

export interface Task {
  id: number
  user_id: number
  title: string
  status: TaskStatus
  reviewed_by: number | null
  reviewed_at: string | null
  created_at: string
}

export interface Notification {
  id: number
  user_id: number
  task_id: number
  message: string
  read: boolean
  created_at: string
}

export interface Webhook {
  id: number
  user_id: number
  url: string
  created_at: string
}

export interface ApiErrorBody {
  status: number
  message: string
}
