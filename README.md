# 口述语料脱敏放行台

口述语料脱敏放行台面向语言档案员、隐私复核员和研究资料管理员，用一个版本化 JSON HTTP API 管理口述语料从建档到研究开放的完整链路。系统支持草稿元数据受控修订和转写片段批量原子登记，按说话人展示授权覆盖与到期风险，以 Unicode rune 区间处理敏感内容，逐项闭环复核退回意见，并在签发前对冻结清单、来源摘要和当前授权重新核验。

## 构建、运行与测试

项目仅使用 Go 标准库，要求 Go 1.23 或兼容的新版本。

```text
go build ./cmd/server
go test ./...
go run ./cmd/server -addr=127.0.0.1:19081
```

默认监听 `127.0.0.1:19081`，数据写入 `./data`。可通过 `-addr=127.0.0.1:<port>` 指定地址，也可在未传 `-addr` 时设置 `PORT`；此时服务绑定 `127.0.0.1:<PORT>`。`-data=<directory>` 可切换持久化目录。

运行有界端到端自检：

```text
go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
```

自检通过公开 HTTP API 创建并修订案卷、登记录音和授权、批量登记并锁定转写、生成脱敏稿，实际执行一次复核退回、显式意见处置与再次送审，然后复核通过、冻结并签发凭据；最后验证冻结内容不可修改及凭据摘要一致，并主动关闭服务。自检使用临时数据目录，不污染业务数据。

## HTTP API

所有响应均为 JSON。写命令使用 `POST /api/v1/cases/{caseID}/commands`，正文包含：

```json
{
  "command": "create_case",
  "expectedRevision": 0,
  "idempotencyKey": "client-unique-key",
  "actor": {"id": "archivist-1", "role": "archivist"},
  "payload": {
    "title": "某语言访谈",
    "languageCode": "und",
    "collectionContext": "田野采集背景",
    "ownerID": "archivist-1"
  }
}
```

支持的命令及角色如下：

- `archivist`：`create_case`、`update_case_metadata`、`add_recording`、`add_consent`、`add_segment`、`batch_add_segments`、`lock_intake`、`set_marks`、`resolve_review_finding`、`generate_redaction`、`submit_review`
- `reviewer`：`decide_review`，结论为 `returned` 或 `approved`
- `manager`：`release`

每条写命令必须携带当前 `expectedRevision` 和不超过 128 字符的 `idempotencyKey`。修订号不匹配返回 `revision_conflict`；同一幂等键重放相同请求会返回缓存结果并标记 `idempotentReplay`，复用于不同请求则返回 `idempotency_mismatch`。

`update_case_metadata` 只在 `draft` 状态接受 `title`、`languageCode`、`collectionContext` 和 `ownerID` 的非空字段补丁。`batch_add_segments` 的 `payload.segments` 每批最多 100 项，任一片段无效时会在 `details.issues` 中返回全部可判定问题且整批不写入。复核退回后，`set_marks` 使用 `findingRefs` 明确关联已处理意见，随后以 `resolve_review_finding` 提交非空 `correctionNote`；仍为 `open` 的意见会逐项阻止再次送审。

只读路由：

- `GET /api/v1/cases/{caseID}`：查询完整案卷、当前阻断项、按说话人稳定排序的 `consentCoverage`，以及 `releasePreview`。仅 `review_approved` 状态会生成拟冻结项目、`manifestDigest` 和可定位的放行阻断项；查询不会推进修订或写审计。
- `GET /api/v1/credentials/{credentialID}/verify`：重新计算冻结清单摘要并校验凭据。
- `GET /healthz`：服务健康检查。

请求正文上限为 1 MiB，`Content-Type` 必须是 `application/json`，未知字段和额外 JSON 值会被拒绝。访问日志只记录请求标识、方法、路径、状态码和耗时，不记录原始转写或脱敏正文。

## 持久化与恢复

`internal/store` 使用 SHA-256 校验的长度前缀事件帧作为事实来源。每次提交先检查预期修订号，再同步追加包含案卷投影和幂等结果的事件，随后原子替换带校验和的投影快照。启动恢复会校验事件全局序号、案卷修订号、帧校验和及聚合不变量，并忽略未完整写入的日志尾帧。

冻结清单写入 `manifests.jsonl`，开放凭据写入 `credentials.jsonl`。两类记录均带内容校验和且只追加；启动时会与事件投影交叉验证，拒绝重复或被篡改的凭据，并补齐已提交事件在异常退出时尚未写完的派生只追加记录。
