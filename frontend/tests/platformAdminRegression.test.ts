import { describe, expect, it } from 'vitest'
import { platformAdminGrantInput, platformAdminRevokeMessage } from '../src/components/PlatformAdminManager'
import type { PlatformAdmin } from '../src/api/types'

describe('平台管理员 RBAC 前端回归', () => {
  it('授权输入按邮箱或用户 ID 生成后端 payload', () => {
    expect(platformAdminGrantInput('  OWNER@Example.COM ', 'OWNER', '  bootstrap  ')).toEqual({
      email: 'owner@example.com',
      role: 'OWNER',
      note: 'bootstrap',
    })

    expect(platformAdminGrantInput('  83dc1854-a2c2-418d-875d-f607081a1e4b ', 'ADMIN', '')).toEqual({
      user_id: '83dc1854-a2c2-418d-875d-f607081a1e4b',
      role: 'ADMIN',
      note: '',
    })
  })

  it('OWNER 撤销提示必须包含高危语义', () => {
    const owner: Pick<PlatformAdmin, 'email' | 'user_id' | 'role'> = {
      user_id: 'u-owner',
      email: 'owner@test.local',
      role: 'OWNER',
    }
    const admin: Pick<PlatformAdmin, 'email' | 'user_id' | 'role'> = {
      user_id: 'u-admin',
      role: 'ADMIN',
    }

    expect(platformAdminRevokeMessage(owner)).toContain('平台 OWNER')
    expect(platformAdminRevokeMessage(owner)).toContain('授权能力')
    expect(platformAdminRevokeMessage(admin)).toBe('确定撤销平台管理员 u-admin？')
  })
})
