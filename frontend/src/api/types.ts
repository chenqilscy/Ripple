// 后端 API 类型定义。与 backend-go 的 JSON 输出对齐。

export interface AuthTokens {
  access_token: string
  token_type: 'Bearer'
  expires_in: number
  user: User
}

export interface User {
  id: string
  email: string
  display_name: string
}

export interface Lake {
  id: string
  name: string
  description: string
  is_public: boolean
  owner_id: string
  role?: 'OWNER' | 'EDITOR' | 'PASSENGER' | 'OBSERVER'
}

export type NodeState = 'MIST' | 'DROP' | 'FROZEN' | 'VAPOR' | 'ERASED' | 'GHOST'
export type NodeType = 'TEXT' | 'IMAGE' | 'AUDIO' | 'VIDEO' | 'LINK'

export interface NodeItem {
  id: string
  lake_id: string
  owner_id: string
  content: string
  type: NodeType
  state: NodeState
  position: { x: number; y: number; z: number }
  created_at: string
  updated_at: string
  ttl_at?: string | null
}

export type CloudStatus = 'queued' | 'running' | 'done' | 'failed'

export interface CloudTask {
  id: string
  owner_id: string
  lake_id: string
  prompt: string
  n: number
  node_type: NodeType
  status: CloudStatus
  retry_count: number
  last_error?: string
  result_node_ids: string[] | null
  created_at: string
  started_at?: string | null
  completed_at?: string | null
}

export interface ApiError extends Error {
  status: number
  code?: string
}
