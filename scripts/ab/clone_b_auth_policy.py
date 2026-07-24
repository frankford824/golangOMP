"""Shared, zero-secret identity policy for isolated Clone B rehearsals."""

from __future__ import annotations

import json
from typing import Any


POLICY: dict[str, Any] = {
    "departments": [
        "人事部",
        "运营部",
        "设计研发部",
        "定制美工部",
        "审核部",
        "云仓部",
        "未分配",
    ],
    "department_teams": {
        "人事部": ["人事管理组"],
        "运营部": [
            "淘系一组",
            "淘系二组",
            "天猫一组",
            "天猫二组",
            "拼多多南京组",
            "拼多多池州组",
        ],
        "设计研发部": ["默认组"],
        "定制美工部": ["默认组"],
        "审核部": ["普通审核组", "定制审核组"],
        "云仓部": ["默认组"],
        "未分配": ["未分配池"],
    },
    "phone_unique": True,
    "department_admin_keys": {},
    "super_admins": [],
    "unassigned_pool_enabled": True,
    "configured_user_assignments": [],
    "task_team_mappings": {},
}


def validate(raw: bytes) -> dict[str, Any]:
    payload = json.loads(raw.decode("utf-8"))
    if (
        payload != POLICY
        or type(payload.get("phone_unique")) is not bool
        or type(payload.get("unassigned_pool_enabled")) is not bool
    ):
        raise ValueError(
            "auth settings must equal the frozen zero-secret Clone B identity policy"
        )
    return payload
