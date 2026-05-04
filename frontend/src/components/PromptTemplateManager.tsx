/**
 * PromptTemplateManager · Prompt 模板库管理
 * 修复：scroll lock + CSS 变量 + 响应式三列 Grid + Catppuccin 移除
 */
import React, { useEffect, useMemo, useState } from 'react'
import { api } from '../api/client'
import type { Organization, PromptScope, PromptTemplate } from '../api/types'
import { Button } from './ui'

type FormState = {
  name: string
  description: string
  template: string
  scope: PromptScope
  orgId: string
}

const emptyForm: FormState = {
  name: '',
  description: '',
  template: '',
  scope: 'private',
  orgId: '',
}

export default function PromptTemplateManager() {
  const [templates, setTemplates] = useState<PromptTemplate[]>([])
  const [organizations, setOrganizations] = useState<Organization[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [form, setForm] = useState<FormState>(emptyForm)

  // Scroll lock
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [])

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const [tplRes, orgRes] = await Promise.allSettled([
        api.listPromptTemplates(),
        api.listOrgs(),
      ])
      if (tplRes.status === 'fulfilled') {
        setTemplates((tplRes.value.items ?? []).slice().sort((left, right) => new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime()))
      } else {
        throw tplRes.reason
      }
      if (orgRes.status === 'fulfilled') {
        setOrganizations(orgRes.value.organizations ?? [])
      } else {
        setOrganizations([])
      }
    } catch (e: any) {
      setError(e?.message ?? '加载失败')
      setTemplates([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  const activeOrgOptions = useMemo(
    () => organizations.filter(org => org.id && org.name),
    [organizations],
  )

  function resetForm() {
    setForm(emptyForm)
    setEditingId(null)
  }

  function beginEdit(template: PromptTemplate) {
    setEditingId(template.id)
    setExpandedId(template.id)
    setForm({
      name: template.name,
      description: template.description ?? '',
      template: template.template,
      scope: template.scope,
      orgId: template.org_id ?? '',
    })
  }

  async function handleSubmit() {
    const normalizedName = form.name.trim()
    const normalizedTemplate = form.template.trim()
    if (!normalizedName || !normalizedTemplate || saving) return
    if (form.scope === 'org' && !form.orgId) {
      setError('共享到组织时必须选择组织')
      return
    }

    setSaving(true)
    setError(null)
    try {
      if (editingId) {
        const updated = await api.updatePromptTemplate(editingId, {
          name: normalizedName,
          description: form.description.trim(),
          template: normalizedTemplate,
        })
        setTemplates(prev => prev.map(item => item.id === editingId ? updated : item))
      } else {
        const created = await api.createPromptTemplate({
          name: normalizedName,
          description: form.description.trim(),
          template: normalizedTemplate,
          scope: form.scope,
          org_id: form.scope === 'org' ? form.orgId : '',
        })
        setTemplates(prev => [created, ...prev])
        setExpandedId(created.id)
      }
      resetForm()
    } catch (e: any) {
      setError(e?.message ?? '保存失败')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(template: PromptTemplate) {
    if (deletingId || saving) return
    if (!window.confirm(`确定删除模板「${template.name}」？`)) return
    setDeletingId(template.id)
    setError(null)
    try {
      await api.deletePromptTemplate(template.id)
      setTemplates(prev => prev.filter(item => item.id !== template.id))
      if (editingId === template.id) resetForm()
      if (expandedId === template.id) setExpandedId(null)
    } catch (e: any) {
      setError(e?.message ?? '删除失败')
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <div style={{ padding: 'var(--space-xl) var(--space-lg)', maxWidth: 960, minWidth: 420, flex: '1 1 620px', display: 'flex', flexDirection: 'column', gap: 'var(--space-md)' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
        <h3 style={{ margin: 0, color: 'var(--text-primary)', fontSize: 'var(--font-xl)', fontWeight: 600 }}>Prompt 模板库</h3>
        <Button variant="primary" size="sm">
          {loading ? '刷新中…' : '刷新'}
        </Button>
      </div>
      <p style={{ margin: 0, color: 'var(--text-tertiary)', fontSize: 'var(--font-md)', lineHeight: 1.6 }}>
        管理 AI Workflow 使用的 Prompt 模板。private 模板仅创建者可见；org 模板组织成员可读取并用于 AI 触发，创建者或组织管理员可维护。
      </p>

      {/* 创建/编辑表单区 */}
      <div style={{
        border: '1px solid var(--border)',
        borderRadius: 'var(--radius-lg)',
        background: 'var(--bg-surface)',
        padding: 'var(--space-lg)',
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 'var(--space-sm)', alignItems: 'center', marginBottom: 'var(--space-md)', flexWrap: 'wrap' }}>
          <strong style={{ color: 'var(--accent)', fontSize: 'var(--font-base)', fontWeight: 600 }}>
            {editingId ? '编辑模板' : '新建模板'}
          </strong>
          {editingId && (
            <Button variant="secondary" size="sm" onClick={resetForm} disabled={saving}>
              取消编辑
            </Button>
          )}
        </div>

        {/* 三列 Grid：名称 / 可见范围 / 组织 */}
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'minmax(180px, 1fr) minmax(160px, 200px) minmax(180px, 1fr)',
          gap: 'var(--space-sm)',
          marginBottom: 'var(--space-sm)',
        }}>
          <input
            value={form.name}
            onChange={event => setForm(prev => ({ ...prev, name: event.target.value }))}
            placeholder="模板名称（必填）"
            disabled={saving || !!deletingId}
            style={inputStyle}
            aria-label="模板名称"
          />
          <select
            value={form.scope}
            onChange={event => setForm(prev => ({ ...prev, scope: event.target.value as PromptScope, orgId: event.target.value === 'org' ? prev.orgId : '' }))}
            disabled={saving || !!deletingId || !!editingId}
            style={inputStyle}
            aria-label="可见范围"
          >
            <option value="private">仅自己可用</option>
            <option value="org">组织共享</option>
          </select>
          <select
            value={form.orgId}
            onChange={event => setForm(prev => ({ ...prev, orgId: event.target.value }))}
            disabled={saving || !!deletingId || form.scope !== 'org' || activeOrgOptions.length === 0 || !!editingId}
            style={inputStyle}
            aria-label="共享到的组织"
          >
            <option value="">{activeOrgOptions.length === 0 ? '暂无可选组织' : '选择共享组织'}</option>
            {activeOrgOptions.map(org => (
              <option key={org.id} value={org.id}>{org.name}</option>
            ))}
          </select>
        </div>

        <input
          value={form.description}
          onChange={event => setForm(prev => ({ ...prev, description: event.target.value }))}
          placeholder="描述（可选）"
          disabled={saving || !!deletingId}
          style={{ ...inputStyle, width: '100%', marginBottom: 'var(--space-sm)' }}
          aria-label="模板描述"
        />

        <textarea
          value={form.template}
          onChange={event => setForm(prev => ({ ...prev, template: event.target.value }))}
          placeholder="模板内容（必填），例如：请根据 {{node_content}} 生成结构化摘要"
          disabled={saving || !!deletingId}
          rows={8}
          style={{ ...inputStyle, width: '100%', minHeight: 180, resize: 'vertical', marginBottom: 'var(--space-md)', fontFamily: 'var(--font-mono)', lineHeight: 1.5 }}
          aria-label="模板内容"
        />

        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 'var(--space-md)', alignItems: 'center', flexWrap: 'wrap' }}>
          <div style={{ fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)' }}>
            常用变量：{'{{node_content}}'}，{'{{lake_name}}'}，{'{{custom_key}}'}
          </div>
          <Button variant="primary" size="sm"
            onClick={() => void handleSubmit()}
            disabled={saving || !!deletingId || !form.name.trim() || !form.template.trim() || (form.scope === 'org' && !form.orgId)}
          >
            {saving ? '保存中…' : editingId ? '保存修改' : '创建模板'}
          </Button>
        </div>
      </div>

      {error && (
        <p style={{ color: 'var(--status-danger)', margin: 0 }}>⚠ {error}</p>
      )}

      {loading ? (
        <p style={{ color: 'var(--text-tertiary)', margin: 0 }}>加载中…</p>
      ) : templates.length === 0 ? (
        <p style={{ color: 'var(--text-tertiary)', margin: 0 }}>暂无 Prompt 模板，先创建一个再接入 AI Workflow。</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)' }}>
          {templates.map(template => {
            const isExpanded = expandedId === template.id
            return (
              <div key={template.id} style={{
                border: '1px solid var(--border)',
                borderRadius: 'var(--radius-lg)',
                background: 'var(--bg-card)',
                padding: 'var(--space-md)',
              }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: 'var(--space-md)', alignItems: 'flex-start', flexWrap: 'wrap' }}>
                  <div style={{ minWidth: 0, flex: '1 1 360px' }}>
                    <div style={{ display: 'flex', gap: 'var(--space-sm)', alignItems: 'center', flexWrap: 'wrap', marginBottom: 'var(--space-xs)' }}>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setExpandedId(prev => prev === template.id ? null : template.id)}
                        style={{ fontWeight: 600, padding: 0, background: 'transparent', border: 'none', color: 'var(--text-primary)', cursor: 'pointer', textAlign: 'left' }}
                        aria-expanded={isExpanded}
                      >
                        {template.name}
                      </Button>
                      <span style={badgeStyleVar(template.scope === 'org' ? 'var(--status-warning)' : 'var(--accent)')}>
                        {template.scope === 'org' ? '组织共享' : '私有'}
                      </span>
                      {template.org_id && (
                        <span style={badgeStyleVar('var(--text-secondary)')}>org</span>
                      )}
                    </div>
                    <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-xs)', marginBottom: 'var(--space-xs)' }}>
                      更新于 {fmtDateTime(template.updated_at)}
                    </div>
                    <div style={{ color: 'var(--text-secondary)', fontSize: 'var(--font-base)', lineHeight: 1.5 }}>
                      {template.description || '无描述'}
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: 'var(--space-sm)', flexWrap: 'wrap' }}>
                    <Button variant="primary" size="sm"
                      onClick={() => beginEdit(template)}
                      disabled={saving || !!deletingId}
                    >
                      编辑
                    </Button>
                    <Button variant="danger" size="sm"
                      onClick={() => void handleDelete(template)}
                      disabled={saving || deletingId === template.id}
                    >
                      {deletingId === template.id ? '删除中…' : '删除'}
                    </Button>
                  </div>
                </div>
                {isExpanded && (
                  <pre style={{
                    margin: 'var(--space-md) 0 0',
                    whiteSpace: 'pre-wrap',
                    color: 'var(--text-primary)',
                    background: 'var(--bg-input)',
                    border: '1px solid var(--border-input)',
                    borderRadius: 'var(--radius-md)',
                    padding: 'var(--space-md)',
                    fontSize: 'var(--font-sm)',
                    lineHeight: 1.6,
                    overflowX: 'auto',
                  }}>
                    {template.template}
                  </pre>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

function fmtDateTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function badgeStyleVar(color: string): React.CSSProperties {
  return {
    color,
    border: `1px solid ${color}`,
    borderRadius: 999,
    padding: '1px var(--space-sm)',
    fontSize: 'var(--font-xs)',
  }
}

const inputStyle: React.CSSProperties = {
  background: 'var(--bg-input)',
  border: '1px solid var(--border-input)',
  borderRadius: 'var(--radius-md)',
  color: 'var(--text-primary)',
  padding: 'var(--space-sm) var(--space-md)',
  fontSize: 'var(--font-base)',
}