import { useEffect, useMemo, useState } from 'react'
import { api } from '../api/client'
import type { Organization, PromptScope, PromptTemplate } from '../api/types'

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
      setError(e?.message ?? 'load failed')
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
      setError(e?.message ?? 'save failed')
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
      setError(e?.message ?? 'delete failed')
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <div style={{ padding: 16, maxWidth: 960, minWidth: 420, flex: '1 1 620px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
        <h3 style={{ margin: 0, color: '#cdd6f4', flex: 1 }}>Prompt 模板库</h3>
        <button onClick={() => void load()} disabled={loading || saving || !!deletingId} style={btnStyle('#89b4fa')}>
          {loading ? '刷新中…' : '刷新'}
        </button>
      </div>
      <p style={{ margin: '0 0 12px', color: '#6c7086', fontSize: 12, lineHeight: 1.6 }}>
        管理 AI Workflow 使用的 Prompt 模板。private 模板仅创建者可见；org 模板组织成员可读取并用于 AI 触发，创建者或组织管理员可维护。
      </p>

      <div style={{ border: '1px solid #313244', borderRadius: 10, background: '#181825', padding: 14, marginBottom: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, alignItems: 'center', marginBottom: 10, flexWrap: 'wrap' }}>
          <strong style={{ color: '#f5c2e7' }}>{editingId ? '编辑模板' : '新建模板'}</strong>
          {editingId && (
            <button onClick={resetForm} disabled={saving} style={btnStyle('#f9e2af', true)}>
              取消编辑
            </button>
          )}
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'minmax(220px, 1fr) minmax(160px, 180px) minmax(220px, 1fr)', gap: 8, marginBottom: 8 }}>
          <input
            value={form.name}
            onChange={event => setForm(prev => ({ ...prev, name: event.target.value }))}
            placeholder="模板名称（必填）"
            disabled={saving || !!deletingId}
            style={inputStyle}
          />
          <select
            value={form.scope}
            onChange={event => setForm(prev => ({ ...prev, scope: event.target.value as PromptScope, orgId: event.target.value === 'org' ? prev.orgId : '' }))}
            disabled={saving || !!deletingId || !!editingId}
            style={selectStyle}
          >
            <option value="private">仅自己可用</option>
            <option value="org">组织共享</option>
          </select>
          <select
            value={form.orgId}
            onChange={event => setForm(prev => ({ ...prev, orgId: event.target.value }))}
            disabled={saving || !!deletingId || form.scope !== 'org' || activeOrgOptions.length === 0 || !!editingId}
            style={selectStyle}
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
          style={{ ...inputStyle, width: '100%', marginBottom: 8 }}
        />

        <textarea
          value={form.template}
          onChange={event => setForm(prev => ({ ...prev, template: event.target.value }))}
          placeholder="模板内容（必填），例如：请根据 {{node_content}} 生成结构化摘要"
          disabled={saving || !!deletingId}
          rows={8}
          style={{ ...inputStyle, width: '100%', minHeight: 180, resize: 'vertical', marginBottom: 10, fontFamily: 'Consolas, monospace', lineHeight: 1.5 }}
        />

        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <div style={{ fontSize: 12, color: '#7f849c' }}>
            常用变量：{'{{node_content}}'}，{'{{lake_name}}'}，{'{{custom_key}}'}
          </div>
          <button
            onClick={() => void handleSubmit()}
            disabled={saving || !!deletingId || !form.name.trim() || !form.template.trim() || (form.scope === 'org' && !form.orgId)}
            style={btnStyle('#a6e3a1')}
          >
            {saving ? '保存中…' : editingId ? '保存修改' : '创建模板'}
          </button>
        </div>
      </div>

      {error && <p style={{ color: '#f38ba8', margin: '0 0 12px' }}>⚠ {error}</p>}

      {loading ? (
        <p style={{ color: '#6c7086' }}>加载中…</p>
      ) : templates.length === 0 ? (
        <p style={{ color: '#6c7086' }}>暂无 Prompt 模板，先创建一个再接入 AI Workflow。</p>
      ) : (
        <div style={{ display: 'grid', gap: 10 }}>
          {templates.map(template => {
            const isExpanded = expandedId === template.id
            return (
              <div key={template.id} style={{ border: '1px solid #313244', borderRadius: 10, background: '#11111b', padding: 14 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'flex-start', flexWrap: 'wrap' }}>
                  <div style={{ minWidth: 0, flex: '1 1 360px' }}>
                    <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap', marginBottom: 6 }}>
                      <button onClick={() => setExpandedId(prev => prev === template.id ? null : template.id)} style={{ ...linkBtnStyle, fontWeight: 600 }}>
                        {template.name}
                      </button>
                      <span style={badgeStyle(template.scope === 'org' ? '#f9e2af' : '#89b4fa')}>
                        {template.scope === 'org' ? '组织共享' : '私有'}
                      </span>
                      {template.org_id && <span style={badgeStyle('#94e2d5')}>org</span>}
                    </div>
                    <div style={{ color: '#7f849c', fontSize: 12, marginBottom: 4 }}>
                      更新于 {fmtDateTime(template.updated_at)}
                    </div>
                    <div style={{ color: '#bac2de', fontSize: 13, lineHeight: 1.5 }}>
                      {template.description || '无描述'}
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                    <button onClick={() => beginEdit(template)} disabled={saving || !!deletingId} style={btnStyle('#89b4fa', true)}>编辑</button>
                    <button onClick={() => void handleDelete(template)} disabled={saving || deletingId === template.id} style={btnStyle('#f38ba8', true)}>
                      {deletingId === template.id ? '删除中…' : '删除'}
                    </button>
                  </div>
                </div>
                {isExpanded && (
                  <pre style={{ margin: '12px 0 0', whiteSpace: 'pre-wrap', color: '#cdd6f4', background: '#181825', border: '1px solid #23233a', borderRadius: 8, padding: 12, fontSize: 12, lineHeight: 1.6, overflowX: 'auto' }}>
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

function btnStyle(color: string, small = false): React.CSSProperties {
  return {
    background: 'transparent',
    border: `1px solid ${color}`,
    color,
    borderRadius: 4,
    padding: small ? '4px 10px' : '5px 12px',
    cursor: 'pointer',
    fontSize: small ? 12 : 13,
  }
}

function badgeStyle(color: string): React.CSSProperties {
  return {
    color,
    border: `1px solid ${color}`,
    borderRadius: 999,
    padding: '1px 8px',
    fontSize: 11,
  }
}

const inputStyle: React.CSSProperties = {
  background: '#1e1e2e',
  border: '1px solid #45475a',
  borderRadius: 4,
  color: '#cdd6f4',
  padding: '8px 10px',
  fontSize: 13,
}

const selectStyle: React.CSSProperties = {
  ...inputStyle,
  minWidth: 160,
}

const linkBtnStyle: React.CSSProperties = {
  background: 'transparent',
  border: 'none',
  color: '#cdd6f4',
  padding: 0,
  cursor: 'pointer',
  textAlign: 'left',
}