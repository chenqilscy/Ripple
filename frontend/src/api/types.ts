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
  space_id?: string
  org_id?: string
  role?: 'OWNER' | 'NAVIGATOR' | 'PASSENGER' | 'OBSERVER'
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

// ---- Notification (P13-B) ----
export interface Notification {
  id: number
  type: string
  payload: Record<string, unknown>
  is_read: boolean
  created_at: string
}

// ---- Edges ----
export type EdgeKind = 'relates' | 'derives' | 'opposes' | 'refines' | 'groups' | 'summarizes' | 'custom'

export interface EdgeItem {
  id: string
  lake_id: string
  src_node_id: string
  dst_node_id: string
  kind: EdgeKind
  label?: string
  owner_id: string
  strength?: number
  created_at: string
  deleted_at?: string | null
}

export interface SummarizeGraphSource {
  id: string
  content_snippet: string
  content_length: number
}

export interface SummarizeGraphEdge {
  source_id: string
  target_id: string
  kind: EdgeKind
}

export interface SummarizeGraphEdgeFailure {
  source_id: string
  target_id: string
  reason: string
}

export interface SummarizeGraphResult {
  summary_node: { id: string; content: string }
  sources: SummarizeGraphSource[]
  edges: SummarizeGraphEdge[]
  edge_failures: SummarizeGraphEdgeFailure[]
  source_count: number
  edge_kind: EdgeKind
  complete: boolean
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

// ---- Spaces (M3-S1) ----
export type SpaceRole = 'OWNER' | 'EDITOR' | 'VIEWER'

export interface Space {
  id: string
  owner_id: string
  name: string
  description: string
  llm_quota_monthly: number
  llm_used_current_month: number
  created_at: string
  updated_at: string
  role?: SpaceRole
}

export interface SpaceMember {
  space_id: string
  user_id: string
  role: SpaceRole
  joined_at: string
  updated_at: string
}

// ---- Perma Nodes (M3-S2) ----
export interface PermaNode {
  id: string
  lake_id: string
  owner_id: string
  title: string
  summary: string
  source_node_ids: string[]
  llm_provider?: string
  llm_cost_tokens?: number
  created_at: string
}

// ---- API Keys (P10-A) ----
export interface APIKeyItem {
  id: string
  org_id?: string
  name: string
  key_prefix: string   // "rpl_XXXXXXXXXXXXXXXX..." (掩码)
  scopes: string[]
  last_used_at?: string | null
  created_at: string
}

export interface APIKeyCreated extends APIKeyItem {
  raw_key: string      // 仅在创建响应中返回一次，请立即保存
}

// ---- Audit Logs (P10-B) ----
export interface AuditLogItem {
  id: string
  actor_id: string
  action: string
  resource_type: string
  resource_id: string
  detail: Record<string, unknown>
  created_at: string
}

export interface GraylistEntry {
  id: string
  email: string
  note: string
  created_by: string
  created_at: string
}

export interface AdminOverviewStats {
  organizations_count: number
  users_count: number
  graylist_entries_count: number
}

export interface AdminOverview {
  stats: AdminOverviewStats
  organizations?: OrgOverview[]
}

export type PlatformAdminRole = 'OWNER' | 'ADMIN'

export interface PlatformAdmin {
  user_id: string
  email?: string
  role: PlatformAdminRole
  note: string
  created_by: string
  created_at: string
}

// ---- Lake Members (P11-C) ----
export type LakeRole = 'OWNER' | 'NAVIGATOR' | 'PASSENGER' | 'OBSERVER'

export interface LakeMember {
  user_id: string
  role: LakeRole
}

// ---- Full-text Search (P12-D) ----
export interface SearchHit {
  node_id: string
  lake_id: string
  snippet: string
  score: number
}

// ---- Organizations (P12-C) ----
export type OrgRole = 'OWNER' | 'ADMIN' | 'MEMBER'

export interface Organization {
  id: string
  name: string
  slug: string
  description: string
  owner_id: string
  created_at: string
  updated_at: string
}

export interface OrgMember {
  org_id: string
  user_id: string
  role: OrgRole
  joined_at: string
}

export interface OrgQuota {
  org_id: string
  max_members: number
  max_lakes: number
  max_nodes: number
  max_attachments: number
  max_api_keys: number
  max_storage_mb: number
  usage?: OrgQuotaUsage
  created_at: string
  updated_at: string
}

export interface OrgQuotaUsage {
  members_used: number
  lakes_used: number
  nodes_used: number
  attachments_used: number
  api_keys_used: number
  storage_mb_used: number
}

export interface OrgOverview {
  organization: Organization
  quota: OrgQuota
  recent_quota_audits?: AuditLogItem[]
}

export type OrgQuotaPatch = Partial<Pick<OrgQuota,
  'max_members' | 'max_lakes' | 'max_nodes' | 'max_attachments' | 'max_api_keys' | 'max_storage_mb'
>>

// ---- P18-A：节点关联推荐 ----
export interface NodeSearchResult {
  node_id: string
  lake_id: string
  snippet: string
  score: number
}

// ---- P18-C：节点模板库 ----
export interface NodeTemplate {
  id: string
  name: string
  description: string
  content: string
  tags: string[]
  is_system: boolean
  created_by: string
  created_at: string
  updated_at: string
}

// ---- P18-D：图谱快照 ----
export interface SnapshotNodeEntry {
  id: string
  title: string
  type?: string
}
export interface SnapshotEdgeEntry {
  id: string
  src: string
  dst: string
  kind?: string
}
export interface SnapshotGraphState {
  nodes: SnapshotNodeEntry[]
  edges: SnapshotEdgeEntry[]
}
export interface LakeSnapshot {
  id: string
  lake_id: string
  name: string
  layout: Record<string, { x: number; y: number }>
  graph_state?: SnapshotGraphState
  created_by: string
  created_at: string
}

// ---- P18-B：节点外链分享 ----
export interface NodeShare {
  id: string
  node_id: string
  token: string
  url: string
  expires_at: string | null
  revoked: boolean
  created_by: string
  created_at: string
}

// ---- Phase 15-A：Prompt 模板 ----
export type PromptScope = 'private' | 'org'

export interface PromptTemplate {
  id: string
  name: string
  description: string
  template: string
  scope: PromptScope
  org_id?: string
  created_by: string
  created_at: string
  updated_at: string
}

// ---- Phase 15-B：订阅套餐 ----
export type BillingCycle = 'monthly' | 'yearly'
export type SubscriptionStatus = 'active' | 'cancelled' | 'expired' | 'trial'

export interface PlanQuotas {
  max_members: number
  max_lakes: number
  max_nodes: number
  max_storage_mb: number
}

export interface SubscriptionPlan {
  id: string
  name: string
  description: string
  price_cny_monthly: number
  price_cny_yearly?: number
  quotas: PlanQuotas
}

export interface OrgSubscription {
  id: string
  org_id: string
  plan_id: string
  status: SubscriptionStatus
  billing_cycle: BillingCycle
  current_period_start: string
  current_period_end: string
  created_at: string
}

// ---- Phase 16：组织真实用量 ----
export interface OrgUsage {
  members: number
  lakes: number
  nodes: number
}

// ---- Phase 15-D：组织 LLM 用量 ----
export interface LLMProviderUsage {
  provider: string
  calls: number
  avg_duration_ms: number
  estimated_cost_cny: number
}

export interface LLMDayUsage {
  date: string
  calls: number
  estimated_cost_cny: number
}

export interface OrgLLMUsage {
  org_id: string
  period_days: number
  total_calls: number
  total_estimated_cost_cny: number
  by_provider: LLMProviderUsage[]
  by_day: LLMDayUsage[]
}

// ---- Phase 15-C：AI Job ----
export type AiJobStatus = 'pending' | 'processing' | 'done' | 'failed'

export interface AiJob {
  ai_job_id?: string
  job_id: string
  node_id: string
  status: AiJobStatus
  progress_pct: number
  estimated_seconds?: number
  error?: string
  started_at?: string | null
  finished_at?: string | null
}

// ---- P20-A：自由文本一键转图谱 ----
export interface ImportTextNodeResult {
  id: string
  content: string
}

export interface ImportTextEdgeResult {
  source_id: string
  target_id: string
  kind: string
}

export interface ImportTextResult {
  nodes: ImportTextNodeResult[]
  edges: ImportTextEdgeResult[]
  imported: number
}

