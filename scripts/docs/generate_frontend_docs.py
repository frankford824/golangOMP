#!/usr/bin/env python3
import argparse
import json
import re
from collections import OrderedDict
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[2]
OPENAPI = ROOT / "docs/api/openapi.yaml"
OUT = ROOT / "docs/frontend"
DOC_REVISION = "V1.3-A2 i_id-first task/ERP/search integration (2026-04-27)"
DOC_SOURCE = "docs/api/openapi.yaml (post V1.3-A2)"

METHODS = ("get", "post", "put", "patch", "delete")
METHOD_LABEL = {
    "get": "GET",
    "post": "POST",
    "put": "PUT",
    "patch": "PATCH",
    "delete": "DELETE",
}

FAMILIES = OrderedDict(
    [
        ("AUTH", ("V1_API_AUTH.md", "认证与登录", "登录、注册、会话身份与密码变更。")),
        ("ME", ("V1_API_ME.md", "当前用户", "当前登录用户、个人资料、组织视图与个人偏好。")),
        ("USERS", ("V1_API_USERS.md", "用户与管理审计", "用户管理、角色、访问规则、权限日志、操作日志与后台日志。")),
        ("ORG", ("V1_API_ORG.md", "组织架构", "部门、团队、组织选项与组织迁移申请。")),
        ("TASKS", ("V1_API_TASKS.md", "任务主流程", "任务创建、列表、详情、模块动作、分派、取消、归档与工作流操作。")),
        ("TASK_ASSETS", ("V1_API_TASK_ASSETS.md", "任务资产中心", "任务内资产中心、创建前参考文件上传与任务参考文件。")),
        ("ASSETS", ("V1_API_ASSETS.md", "资产资源库", "资产检索、详情、下载、预览、上传会话、归档与恢复。")),
        ("DRAFTS", ("V1_API_DRAFTS.md", "任务草稿", "草稿创建、读取、删除、7 天过期与 20 条上限。")),
        ("NOTIFICATIONS", ("V1_API_NOTIFICATIONS.md", "通知", "站内通知列表、已读、全部已读、未读数与 5 类通知事件。")),
        ("BATCH", ("V1_API_BATCH.md", "Excel 批量创建", "批量创建模板下载、Excel 解析与前端预览校验。")),
        ("ERP", ("V1_API_ERP.md", "ERP 与业务字典", "ERP 商品、分类、仓库、同步、类目、成本规则与兼容商品目录。")),
        ("SEARCH", ("V1_API_SEARCH.md", "搜索", "全局搜索、资产搜索与设计来源搜索。")),
        ("REPORTS", ("V1_API_REPORTS.md", "L1 报表", "L1 卡片、吞吐与模块停留报表。")),
        ("WS", ("V1_API_WS.md", "WebSocket", "实时消息连接与事件通道。")),
    ]
)

FAMILY_NOTES = {
    "AUTH": [
        "公开端点仅限注册、登录、注册选项；其余端点需要 Bearer token。",
        "登录成功后，前端统一使用 `Authorization: Bearer <token>`。",
    ],
    "ME": [
        "当前用户 family 只面向当前 token，不应用于管理其他用户。",
        "通知路径拆到 `V1_API_NOTIFICATIONS.md`，避免重复接入。",
    ],
    "USERS": [
        "用户管理端点受管理范围控制，前端必须展示后端返回的 `deny_code`。",
        "角色与访问规则主要供后台管理页使用。",
    ],
    "ORG": [
        "组织字段以前端选择器为主，候选值优先来自 `/v1/org/options`。",
        "组织迁移申请是流程化操作，不要直接修改受管组织字段绕过审批。",
    ],
    "TASKS": [
        "`GET /v1/tasks/{id}/detail` 是 V1.1-A1 优化后的首屏聚合接口，生产 warm P99 约 32.933ms。",
        "任务主流程读接口已统一为 task-facing 登录角色全量可见；接单、编辑、审核、上传、归档等动作仍以后端返回的权限/状态判定为准。",
        "创建任务时前端应优先提交 `i_id`；`category_code` 是后端兼容字段，不作为新前端必填项。",
        "`sync_erp_on_create=true` 时，后端会在创建后用产品名称、SKU 与 i_id 触发前置 ERP upsert。",
        "模块动作按后端工作流状态机判定，前端不要本地推断可执行性作为最终权限。",
    ],
    "TASK_ASSETS": [
        "新联调优先使用 `/v1/tasks/{id}/asset-center/*`。",
        "`/v1/task-create/asset-center/*` 属兼容保留；新前端仅在回滚场景使用。",
    ],
    "ASSETS": [
        "资产上传建议走 upload session；下载与预览 URL 以接口返回为准。",
        "删除、归档、恢复动作需按返回错误处理竞态和权限失败。",
    ],
    "DRAFTS": [
        "草稿有 7 天过期与 20 条上限，前端保存失败时应提示用户清理旧草稿。",
        "草稿 payload 由后端持久化，前端不要假设旧草稿一定符合最新创建表单。",
    ],
    "NOTIFICATIONS": [
        "V1 冻结 5 类通知类型：task_assigned_to_me、task_rejected、claim_conflict、pool_reassigned、task_cancelled。",
        "未读数用于 badge，列表分页以接口返回 cursor/limit 字段为准。",
    ],
    "BATCH": [
        "批量创建只做模板下载和解析预览，不直接写任务表。",
        "Excel 字段与枚举以模板中的 Schema/EnumDict sheet 和接口 violations 为准。",
    ],
    "ERP": [
        "新联调优先使用 `/v1/erp/products*` 与 `/v1/erp/products/by-code`。",
        "`/v1/erp/iids` 是新建/采购任务选择聚水潭 i_id 的 canonical 入口。",
        "`/v1/products*` 是兼容本地缓存路径，新前端不要作为主入口。",
    ],
    "SEARCH": [
        "搜索接口是只读入口，低权限用户可能拿到空数组而不是错误。",
        "`GET /v1/search` 的任务搜索覆盖任务号、产品名、SKU、i_id、任务类型、创建人、所属组、设计师、日期与任务关联设计图/参考图文件信息。",
        "高频输入框应做前端 debounce，避免无意义请求。",
    ],
    "REPORTS": [
        "L1 报表仅 super_admin 可用。",
        "403 时重点展示 `reports_super_admin_only`。",
    ],
    "WS": [
        "WebSocket 使用 Bearer token 建连；断线后前端按指数退避重连。",
        "实时事件只是提醒，页面最终状态仍以 HTTP detail/list 结果为准。",
    ],
}

COMMON_ERRORS = [
    ("401", "UNAUTHENTICATED", "-", "未登录、token 缺失或 token 过期。"),
    ("403", "PERMISSION_DENIED", "见接口返回", "角色、组织范围、字段级授权或流程状态不允许。"),
    ("404", "NOT_FOUND", "-", "资源不存在或当前用户不可见。"),
    ("409", "CONFLICT", "见接口返回", "状态竞态、重复操作或版本冲突。"),
    ("422", "VALIDATION_ERROR", "-", "请求参数或业务字段校验失败。"),
    ("500", "INTERNAL", "-", "后端内部错误；联调时带 trace/log 找后端排查。"),
]

DENY_CODES = [
    "task_create_field_denied_by_scope",
    "task_out_of_scope",
    "task_out_of_stage_scope",
    "task_not_assigned_to_actor",
    "task_status_not_actionable",
    "task_not_reassignable",
    "module_action_role_denied",
    "department_scope_only",
    "team_scope_only",
    "org_admin_scope_only",
    "user_update_field_denied_by_scope",
    "role_assignment_denied_by_scope",
    "management_access_required",
    "reports_super_admin_only",
    "asset_version_race_retry",
    "audit_log_access_denied",
    "workflow_lane_unsupported",
    "old_password_mismatch",
    "password_confirmation_required",
    "password_confirmation_mismatch",
]


def load_spec():
    with OPENAPI.open(encoding="utf-8") as f:
        return yaml.safe_load(f)


def ref_name(ref):
    return ref.rsplit("/", 1)[-1]


def resolve(spec, schema, depth=0):
    if not isinstance(schema, dict):
        return schema
    if "$ref" in schema:
        name = ref_name(schema["$ref"])
        target = spec.get("components", {}).get("schemas", {}).get(name, {})
        merged = dict(target)
        for k, v in schema.items():
            if k != "$ref":
                merged[k] = v
        merged.setdefault("x-schema-name", name)
        return merged if depth > 2 else resolve(spec, merged, depth + 1)
    return schema


def schema_type(spec, schema):
    schema = resolve(spec, schema or {})
    if not isinstance(schema, dict):
        return "object"
    if "x-schema-name" in schema:
        base = schema["x-schema-name"]
    else:
        base = schema.get("type") or ("object" if "properties" in schema else "any")
    if base == "array":
        item = resolve(spec, schema.get("items", {}))
        if isinstance(item, dict) and item.get("x-schema-name"):
            return f"array<{item['x-schema-name']}>"
        return f"array<{schema_type(spec, item)}>"
    enum = schema.get("enum")
    if enum:
        vals = "/".join(str(v) for v in enum[:12])
        more = "..." if len(enum) > 12 else ""
        return f"enum({vals}{more})"
    return base


def schema_desc(schema):
    if not isinstance(schema, dict):
        return ""
    desc = schema.get("description") or schema.get("title") or ""
    return " ".join(str(desc).split())


def schema_table(spec, schema):
    schema = resolve(spec, schema or {})
    if not isinstance(schema, dict):
        return "| 字段 | 类型 | 必填 | 说明 |\n|---|---|---|---|\n| `body` | any | 否 | OpenAPI 未声明结构化 schema。 |\n"
    if schema.get("type") == "array":
        return "| 字段 | 类型 | 必填 | 说明 |\n|---|---|---|---|\n| `body` | %s | 是 | 数组请求或响应体。 |\n" % schema_type(spec, schema)
    props = schema.get("properties") or {}
    required = set(schema.get("required") or [])
    if not props:
        typ = schema_type(spec, schema)
        desc = schema_desc(schema) or "OpenAPI 声明的整体对象。"
        return "| 字段 | 类型 | 必填 | 说明 |\n|---|---|---|---|\n| `body` | %s | 视接口 | %s |\n" % (typ, desc)
    rows = ["| 字段 | 类型 | 必填 | 说明 |", "|---|---|---|---|"]
    for name, prop in props.items():
        p = resolve(spec, prop)
        required_label = "是" if name in required else "否"
        desc = schema_desc(p) or "-"
        rows.append(f"| `{name}` | {schema_type(spec, p)} | {required_label} | {desc} |")
    return "\n".join(rows) + "\n"


def example_for_schema(spec, schema, depth=0):
    schema = resolve(spec, schema or {})
    if depth > 2:
        return "..."
    if not isinstance(schema, dict):
        return {}
    enum = schema.get("enum")
    if enum:
        return enum[0]
    typ = schema.get("type")
    if typ == "array":
        return [example_for_schema(spec, schema.get("items", {}), depth + 1)]
    if typ == "integer":
        return 123
    if typ == "number":
        return 12.3
    if typ == "boolean":
        return True
    if typ == "string":
        fmt = schema.get("format")
        if fmt == "date-time":
            return "2026-04-25T10:30:41Z"
        if fmt == "date":
            return "2026-04-25"
        return "string"
    props = schema.get("properties") or {}
    if props:
        required = schema.get("required") or list(props)[:4]
        return {k: example_for_schema(spec, props[k], depth + 1) for k in list(required)[:6] if k in props}
    return {}


def operation_schema(spec, op, direction):
    if direction == "request":
        body = op.get("requestBody") or {}
        content = body.get("content") or {}
        for media in ("application/json", "multipart/form-data", "application/octet-stream"):
            if media in content:
                return media, content[media].get("schema") or {}
        if content:
            media, cfg = next(iter(content.items()))
            return media, cfg.get("schema") or {}
        return None, None
    responses = op.get("responses") or {}
    for status in ("200", "201", "204", "default"):
        resp = responses.get(status)
        if not resp:
            continue
        content = resp.get("content") or {}
        if not content:
            return status, {}
        for media in ("application/json", "application/octet-stream", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"):
            if media in content:
                return f"{status} {media}", content[media].get("schema") or {}
        media, cfg = next(iter(content.items()))
        return f"{status} {media}", cfg.get("schema") or {}
    return None, None


def params_table(spec, path_item, op):
    params = []
    for p in path_item.get("parameters") or []:
        if isinstance(p, dict):
            params.append(p)
    for p in op.get("parameters") or []:
        if isinstance(p, dict):
            params.append(p)
    if not params:
        return "无 path/query/header 参数。\n"
    rows = ["| 参数 | 位置 | 类型 | 必填 | 说明 |", "|---|---|---|---|---|"]
    for p in params:
        schema = p.get("schema") or {}
        rows.append(
            f"| `{p.get('name','-')}` | {p.get('in','-')} | {schema_type(spec, schema)} | {'是' if p.get('required') else '否'} | {schema_desc(p) or schema_desc(schema) or '-'} |"
        )
    return "\n".join(rows) + "\n"


def collect_roles(op):
    parts = []
    for key in ("x-rbac-placeholder", "x-rbac", "x-roles"):
        val = op.get(key)
        if not val:
            continue
        if isinstance(val, dict):
            allowed = val.get("allowed_roles") or val.get("roles") or val.get("allowed")
            if allowed:
                parts.extend(str(x) for x in allowed)
        elif isinstance(val, list):
            parts.extend(str(x) for x in val)
        else:
            parts.append(str(val))
    sec = op.get("security")
    if sec == []:
        return "公开"
    if parts:
        return ", ".join(dict.fromkeys(parts))
    return "已登录 / scope-aware"


def access_label(path, method, op):
    if method == "get" and path in {"/v1/tasks", "/v1/tasks/{id}", "/v1/tasks/{id}/detail"}:
        return "已登录 / 主流程读全量可见"
    return collect_roles(op)


def error_rows(op):
    rows = ["| HTTP | code | deny_code | 说明 |", "|---|---|---|---|"]
    responses = op.get("responses") or {}
    found = False
    for status, resp in responses.items():
        if not str(status).startswith(("4", "5")):
            continue
        found = True
        desc = " ".join(str(resp.get("description") or "").split()) or "错误响应。"
        rows.append(f"| {status} | 见 `error.code` | 见 `deny_code` | {desc} |")
    if not found:
        for row in COMMON_ERRORS[:5]:
            rows.append("| %s | %s | %s | %s |" % row)
    return "\n".join(rows) + "\n"


def curl_for(path, method, op, media):
    url = "https://api.example.com" + re.sub(r"\{([^}]+)\}", r"<\1>", path)
    lines = [f"curl -X {METHOD_LABEL[method]} {url} \\", "  -H \"Authorization: Bearer $TOKEN\""]
    if method in ("post", "put", "patch"):
        if media == "multipart/form-data":
            lines.append("  -F \"file=@example.xlsx\"")
        elif media:
            lines[-1] += " \\"
            lines.append("  -H \"Content-Type: application/json\" \\")
            lines.append("  -d '{\"example\":\"value\"}'")
    return "\n".join(lines)


def family_for(path):
    if path.startswith("/v1/ws"):
        return "WS"
    if path.startswith("/v1/reports/"):
        return "REPORTS"
    if path in ("/v1/search", "/v1/assets/search", "/v1/design-sources/search"):
        return "SEARCH"
    if path.startswith("/v1/tasks/batch-create"):
        return "BATCH"
    if path.startswith("/v1/me/notifications"):
        return "NOTIFICATIONS"
    if path.startswith("/v1/task-drafts"):
        return "DRAFTS"
    if path.startswith("/v1/tasks/{id}/asset-center") or path.startswith("/v1/task-create/asset-center") or path == "/v1/tasks/reference-upload":
        return "TASK_ASSETS"
    if path.startswith("/v1/assets"):
        return "ASSETS"
    if path.startswith("/v1/auth"):
        return "AUTH"
    if path.startswith("/v1/me"):
        return "ME"
    if path.startswith("/v1/users") or path in ("/v1/roles", "/v1/access-rules", "/v1/permission-logs", "/v1/operation-logs", "/v1/audit-logs", "/v1/server-logs", "/v1/server-logs/clean") or path.startswith("/v1/admin/jst-users"):
        return "USERS"
    if path.startswith("/v1/org") or path.startswith("/v1/departments"):
        return "ORG"
    if path.startswith("/v1/tasks"):
        return "TASKS"
    if path.startswith("/v1/erp") or path.startswith("/v1/products") or path.startswith("/v1/categories") or path.startswith("/v1/category-mappings") or path.startswith("/v1/cost-rules"):
        return "ERP"
    return "TASKS"


def path_summary(path_item):
    bits = []
    for m in METHODS:
        op = path_item.get(m)
        if isinstance(op, dict):
            bits.append(op.get("summary") or f"{METHOD_LABEL[m]} endpoint")
    return "；".join(bits) if bits else "OpenAPI path"


def render_path_section(spec, path, path_item, family_key):
    ops = [(m, path_item[m]) for m in METHODS if isinstance(path_item.get(m), dict)]
    primary_method = ops[0][0]
    methods = ", ".join(METHOD_LABEL[m] for m, _ in ops)
    title = f"## {METHOD_LABEL[primary_method]} {path}\n\n"
    out = [title]
    out.append("### 简介\n")
    desc = []
    for m, op in ops:
        text = op.get("description") or op.get("summary") or f"{METHOD_LABEL[m]} {path}"
        desc.append(f"- `{METHOD_LABEL[m]}`: {' '.join(str(text).split())}")
    out.append(f"支持方法: {methods}。\n\n" + "\n".join(desc) + "\n\n")
    out.append("### 鉴权与 RBAC\n")
    out.append("- 需要 Bearer token(`Authorization: Bearer <token>`)，除非本节标为公开。\n")
    role_lines = [f"- `{METHOD_LABEL[m]}` 允许角色: {access_label(path, m, op)}。" for m, op in ops]
    out.append("\n".join(role_lines) + "\n")
    out.append("- 字段级授权: 以后端返回的 `error.code` / `deny_code` 为准。\n\n")

    for m, op in ops:
        prefix = "" if len(ops) == 1 else f"#### {METHOD_LABEL[m]} 细节\n\n"
        if prefix:
            out.append(prefix)
        out.append("### 请求体 schema\n" if len(ops) == 1 else "##### 请求体 schema\n")
        out.append("参数:\n\n")
        out.append(params_table(spec, path_item, op) + "\n")
        media, req_schema = operation_schema(spec, op, "request")
        if req_schema is None:
            out.append("请求体: 无请求体。\n\n")
        else:
            out.append(f"Content-Type: `{media}`\n\n")
            out.append(schema_table(spec, req_schema) + "\n")
        out.append("### 响应体 schema\n" if len(ops) == 1 else "##### 响应体 schema\n")
        resp_label, resp_schema = operation_schema(spec, op, "response")
        out.append(f"成功响应: `{resp_label or '见 OpenAPI responses'}`\n\n")
        if resp_schema:
            out.append("```json\n")
            out.append(json.dumps(example_for_schema(spec, resp_schema), ensure_ascii=False, indent=2)[:1800])
            out.append("\n```\n\n")
            out.append(schema_table(spec, resp_schema) + "\n")
        else:
            out.append("无 JSON 响应体或响应体由文件流承载。\n\n")
        out.append("### 错误码\n" if len(ops) == 1 else "##### 错误码\n")
        out.append(error_rows(op) + "\n")
        out.append("### curl 示例\n" if len(ops) == 1 else "##### curl 示例\n")
        out.append("```bash\n" + curl_for(path, m, op, media) + "\n```\n\n")
    out.append("### 前端最佳实践\n")
    for note in FAMILY_NOTES[family_key]:
        out.append(f"- {note}\n")
    out.append("- 优先用 canonical 路径；兼容或 deprecated 路径仅用于迁移兜底。\n")
    out.append("- 失败时必须展示 `error.code` 或 `deny_code`，不要只显示 HTTP 状态码。\n\n")
    return "".join(out)


def revision_header():
    return f"> Revision: {DOC_REVISION}\n> Source: {DOC_SOURCE}\n\n"


def render_family(spec, key, entries, v1_path_count):
    filename, title, intro = FAMILIES[key]
    out = [f"# {title}\n\n"]
    out.append(revision_header())
    out.append("> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。\n\n")
    out.append(f"{intro}\n\n")
    out.append("## Family 约定\n\n")
    for note in FAMILY_NOTES[key]:
        out.append(f"- {note}\n")
    out.append(f"- 本文件覆盖 `{len(entries)}` 个 `/v1` path；同一路径多 method 合并在同一节。\n\n")
    for path, item in entries:
        out.append(render_path_section(spec, path, item, key))
    if key == "WS":
        ws_item = spec.get("paths", {}).get("/ws/v1")
        if ws_item:
            out.append("## GET /ws/v1\n\n")
            out.append("### 简介\n")
            out.append(f"当前 OpenAPI 实际挂载的 WebSocket path 是 `/ws/v1`，不是 `/v1/ws/v1`。本文按 OpenAPI 真实路径记录；`/v1` path 统计为 {v1_path_count}。\n\n")
            out.append("### 鉴权与 RBAC\n")
            out.append("- 需要 Bearer token，推荐通过协议约定或查询参数传递，具体以 transport 实现和前端联调环境为准。\n- 允许角色: 已登录用户。\n- 字段级授权: 无。\n\n")
            out.append("### 请求体 schema\n无 HTTP JSON 请求体；WebSocket 握手后收发消息。\n\n")
            out.append("### 响应体 schema\n成功握手返回 101 Switching Protocols；消息体以服务端事件 JSON 为准。\n\n")
            out.append("### 错误码\n| HTTP | code | deny_code | 说明 |\n|---|---|---|---|\n| 401 | UNAUTHENTICATED | - | token 缺失或过期。 |\n| 403 | PERMISSION_DENIED | - | 当前账号无实时通道权限。 |\n\n")
            out.append("### curl 示例\n```bash\ncurl -i -N https://api.example.com/ws/v1 \\\n  -H \"Authorization: Bearer $TOKEN\" \\\n  -H \"Connection: Upgrade\" \\\n  -H \"Upgrade: websocket\"\n```\n\n")
            out.append("### 前端最佳实践\n- 使用指数退避重连。\n- WebSocket 事件只作为刷新提示，最终页面状态回读 HTTP 接口。\n\n")
    return filename, "".join(out)


def render_index(family_entries, v1_path_count):
    out = ["# V1 前端联调接口文档索引\n\n"]
    out.append(revision_header())
    out.append("当前真相入口: [V1_BACKEND_SOURCE_OF_TRUTH.md](../V1_BACKEND_SOURCE_OF_TRUTH.md)\n\n")
    out.append("> Release: v1.21 · Backend: V1.0 + V1.1-A1 · Production detail P99 warm 32.933ms / cold 32.995ms。\n\n")
    out.append("## §0 Base URL 与鉴权\n\n")
    out.append("- 生产: `https://<prod-host>` 或联调反代地址。\n- 本地/隧道: `http://127.0.0.1:18080`。\n- 鉴权: `Authorization: Bearer <token>`。\n- 成功响应常见包装: `{\"data\": ...}`；以各接口 OpenAPI response schema 为准。\n\n")
    out.append("## §1 联调起步 6 步\n\n")
    out.append("1. `POST /v1/auth/login` 获取 token。\n2. `GET /v1/me` 校验当前用户。\n3. `GET /v1/tasks` 拉任务列表。\n4. `GET /v1/tasks/{id}/detail` 拉首屏聚合详情。\n5. 使用 `/v1/tasks/{id}/asset-center/*` 联调任务资产。\n6. 使用 `/v1/tasks/batch-create/template.xlsx` 与 `/parse-excel` 联调 Excel 批量预览。\n\n")
    out.append("## §2 错误码总表\n\n")
    out.append("| HTTP | code | deny_code | 说明 |\n|---|---|---|---|\n")
    for row in COMMON_ERRORS:
        out.append("| %s | %s | %s | %s |\n" % row)
    out.append("\n常见 deny_code:\n\n")
    for code in DENY_CODES:
        out.append(f"- `{code}`\n")
    out.append("\n## §3 RBAC 角色矩阵\n\n")
    out.append("| 角色 | 主要权限点 |\n|---|---|\n")
    roles = [
        ("SuperAdmin", "全局管理、报表、危险操作、用户管理。"),
        ("HRAdmin", "组织与用户管理范围内操作。"),
        ("DepartmentAdmin", "本部门用户与任务管理。"),
        ("TeamLead", "本组任务管理与人员协作。"),
        ("Ops", "运营/客服任务创建、分派与跟进。"),
        ("Designer", "设计模块领取、提交与资产处理。"),
        ("CustomizationOperator", "定制模块处理。"),
        ("Audit_A / Audit_B / CustomizationReviewer", "审核相关模块动作。"),
        ("Warehouse / Member", "仓库或普通成员范围内可见任务与操作。"),
    ]
    for role, text in roles:
        out.append(f"| `{role}` | {text} |\n")
    out.append("\n## §4 路由分类\n\n")
    out.append("- Canonical: `/v1/auth/*`, `/v1/me*`, `/v1/users*`, `/v1/erp/products*`, `/v1/tasks*`, `/v1/tasks/{id}/asset-center/*`, `/v1/task-drafts*`, `/v1/me/notifications*`, `/v1/reports/l1/*`, `/ws/v1`。\n")
    out.append("- Compatibility: `/v1/products*`, `/v1/task-create/asset-center/*`, 以及 transport 中 `withCompatibilityRoute` 标记的旧入口。\n")
    out.append("- Deprecated: transport 中 `withDeprecatedRoute` 标记的旧入口；新前端不要接。\n\n")
    out.append("## §5 Family 索引\n\n")
    out.append("| Family | 文档 | path 数 |\n|---|---|---|\n")
    for key, entries in family_entries.items():
        filename, title, _ = FAMILIES[key]
        count = str(len(entries))
        if key == "WS":
            count = "0 个 `/v1` path + `/ws/v1`"
        out.append(f"| {title} | [{filename}]({filename}) | {count} |\n")
    out.append(f"| 全量速查 | [V1_API_CHEATSHEET.md](V1_API_CHEATSHEET.md) | {v1_path_count} |\n\n")
    out.append("## §6 联调硬门\n\n")
    out.append("- 所有请求必须走 Bearer token，公开登录/注册除外。\n- 首屏详情优先使用 `GET /v1/tasks/{id}/detail`，不要并发拼旧 detail 子接口。\n- 前端必须展示后端 `error.code` 或 `deny_code`。\n- 新页面只接 canonical 路径。\n- WebSocket 只做实时提示，最终一致状态回读 HTTP。\n- Excel 批量创建以 parse preview 的 `violations` 为准，不在前端复制完整业务校验。\n\n")
    out.append("## §7 Deprecated / Compatibility 清单\n\n")
    out.append("- `/v1/task-create/asset-center/*`: 创建前资产上传兼容入口。\n- `/v1/products*`: 老本地缓存商品入口，新联调用 `/v1/erp/products*`。\n- `/v1/tasks/{id}/audit_a_claim`、`/v1/tasks/{id}/audit_b_claim`: 老审核领取别名。\n- 所有 `withCompatibilityRoute` / `withDeprecatedRoute` 标记路径不得作为新前端主入口。\n\n")
    return "".join(out)


def render_cheatsheet(spec, family_entries, v1_path_count):
    rows = [f"# V1 API 速查表({v1_path_count} path · 一行一条)\n\n"]
    rows.append(revision_header())
    rows.append("> 本表一行对应一个 `/v1` path；同一路径多 method 合并到 `Methods` 列。\n")
    rows.append(f"> WebSocket 当前 OpenAPI 真实 path 为 `/ws/v1`，详见 `V1_API_WS.md`，不计入 {v1_path_count} 个 `/v1` path。\n")
    rows.append("> 新前端只接 canonical 路径；compatibility/deprecated 路径仅作迁移兜底。\n\n")
    rows.append("| Methods | Path | Summary | RBAC | family doc |\n|---|---|---|---|---|\n")
    for key, entries in family_entries.items():
        filename, _, _ = FAMILIES[key]
        for path, item in entries:
            ops = [(m, item[m]) for m in METHODS if isinstance(item.get(m), dict)]
            methods = ", ".join(METHOD_LABEL[m] for m, _ in ops)
            summary = path_summary(item).replace("|", "\\|")
            roles = "; ".join(f"{METHOD_LABEL[m]}:{access_label(path, m, op)}" for m, op in ops).replace("|", "\\|")
            rows.append(f"| {methods} | `{path}` | {summary} | {roles} | [{filename}]({filename}) |\n")
    return "".join(rows)


def parse_args():
    parser = argparse.ArgumentParser(description="Generate V1 frontend API docs from docs/api/openapi.yaml.")
    parser.add_argument("--dry-run", action="store_true", help="Render and print the path distribution without writing files.")
    parser.add_argument("--expect-v1-path-count", type=int, default=None, help="Optional explicit guard for the current /v1 path count.")
    return parser.parse_args()


def main():
    args = parse_args()
    spec = load_spec()
    OUT.mkdir(parents=True, exist_ok=True)
    paths = [(p, item) for p, item in spec["paths"].items() if p.startswith("/v1")]
    v1_path_count = len(paths)
    if args.expect_v1_path_count is not None and v1_path_count != args.expect_v1_path_count:
        raise SystemExit(f"expected {args.expect_v1_path_count} /v1 paths, got {v1_path_count}")
    family_entries = OrderedDict((key, []) for key in FAMILIES)
    for path, item in paths:
        family_entries[family_for(path)].append((path, item))
    covered = sum(len(v) for v in family_entries.values())
    if covered != v1_path_count:
        raise SystemExit(f"coverage mismatch: covered {covered}, paths {v1_path_count}")
    rendered = OrderedDict()
    for key, entries in family_entries.items():
        filename, text = render_family(spec, key, entries, v1_path_count)
        rendered[filename] = text
    rendered["INDEX.md"] = render_index(family_entries, v1_path_count)
    rendered["V1_API_CHEATSHEET.md"] = render_cheatsheet(spec, family_entries, v1_path_count)
    if not args.dry_run:
        for filename, text in rendered.items():
            (OUT / filename).write_text(text, encoding="utf-8")
    action = "would generate" if args.dry_run else "generated"
    print(f"{action} {len(rendered)} files; /v1 paths={v1_path_count}")
    for key, entries in family_entries.items():
        print(f"{FAMILIES[key][0]} {len(entries)}")


if __name__ == "__main__":
    main()
