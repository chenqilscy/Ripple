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

// ---- Edges ----
export type EdgeKind = 'relates' | 'derives' | 'opposes' | 'refines' | 'groups' | 'custom'

export interface EdgeItem {
  id: string
  lake_id: string
  src_node_id: string
  dst_node_id: string
  kind: EdgeKind
  label?: string
  owner_id: string
  created_at: string
  deleted_at?: string | null
}

// ---- Invites ----
export interface InviteItem {
  id: string
  lake_id: string
  token: string
  created_by: string
  role: 'NAVIGATOR' | 'PASSENGER' | 'OBSERVER'
  max_uses: number
  used_count: number
  expires_at: string
  revoked_at?: string | null
  created_at: string
}

export interface InvitePreview {
  lake_id: string
  lake_name: string
  inviter_id: string
  role: string
  expires_at: string
  used_count: number
  max_uses: number
  alive: boolean
}

// ---- Node Revisions (F3) ----
export interface NodeRevision {
  id: string
  node_id: string
  rev_number: number
  content: string
  title: string
  editor_id: string
  edit_reason: string
  created_at: string
}
