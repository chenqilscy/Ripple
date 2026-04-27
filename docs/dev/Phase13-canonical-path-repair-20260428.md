# Phase 13 canonical 路径修复记录（2026-04-28）

## 背景

Phase 13 回滚验收后，staging 服务从 `/home/admin/Ripple-rollback-current` 运行。尝试恢复 canonical `/home/admin/Ripple` 时发现旧目录存在 root-owned 文件，导致普通 `admin` 用户无法删除或覆盖。

## 处理原则

- 不触碰当前运行容器；
- 不删除数据卷；
- 不静默 `rm -rf`；
- 先备份旧目录，再重建 canonical 源码路径。

## 执行结果

1. 本地生成最新 `main` 源码包：`ripple-121d7c4.tar`；
2. 上传到远端 `/home/admin/ripple-121d7c4.tar`；
3. 使用 `sudo chown -R admin:Users /home/admin/Ripple` 修复旧目录权限；
4. 将旧目录备份为：`/home/admin/Ripple-canonical-backup-20260428-003705`；
5. 重新创建 `/home/admin/Ripple` 并解包最新源码；
6. 检查 `/home/admin/Ripple` 下无非 `admin` owner 文件；
7. 执行 smoke，当前 staging 服务未受影响。

Smoke：

```text
OK phase13 smoke passed
lake_id=26acb427-efd5-4201-b084-f9b0a103f47a
node_id=ac64e0aa-6e79-4d44-809b-2d2a2a6feffa
org_id=ba0697b2-dc45-4447-b330-884d241df10a
```

## 当前状态

- `/home/admin/Ripple` 已恢复为 admin 可写的标准源码路径；
- 当前运行服务仍保持从 `/home/admin/Ripple-rollback-current` 启动，未在本次操作中切换；
- 后续若需要把运行目录切回 canonical，应在独立窗口执行 compose down/up 并重新 smoke。
