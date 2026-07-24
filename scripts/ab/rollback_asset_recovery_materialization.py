#!/usr/bin/env python3
"""Remove only run-scoped recovery objects recorded by a materialization plan."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import tempfile
from typing import Any


SHA256 = re.compile(r"^[0-9a-f]{64}$")
RUN_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{2,80}$")


def canonical_bytes(value: Any) -> bytes:
    return (
        json.dumps(
            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        + "\n"
    ).encode("utf-8")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def contained(root: pathlib.Path, key: str) -> pathlib.Path:
    relative = pathlib.PurePosixPath(key)
    if (
        relative.is_absolute()
        or not relative.parts
        or any(part in {"", ".", ".."} for part in relative.parts)
    ):
        raise ValueError("recovery object key is unsafe")
    target = root.joinpath("objects", *relative.parts)
    try:
        target.resolve(strict=False).relative_to(root)
    except ValueError:
        raise ValueError("recovery object escapes fixture root") from None
    if target.is_symlink():
        raise ValueError("symlinked recovery objects are forbidden")
    return target


def rollback(plan: dict[str, Any], fixture_root: pathlib.Path) -> dict[str, Any]:
    if (
        plan.get("version") != 1
        or plan.get("status") != "MATERIALIZED"
        or plan.get("database_writes_executed") is not False
        or plan.get("production_writes_executed") is not False
    ):
        raise ValueError("recovery plan is not an exact file-only materialization")
    run_id = str(plan.get("run_id") or "")
    if not RUN_ID.fullmatch(run_id):
        raise ValueError("recovery plan run_id is invalid")
    entries = plan.get("entries")
    if not isinstance(entries, list) or not entries:
        raise ValueError("recovery plan entries are missing")
    targets: list[tuple[pathlib.Path, str, int, str]] = []
    seen: set[pathlib.Path] = set()
    for index, entry in enumerate(entries):
        if not isinstance(entry, dict):
            raise ValueError(f"entries[{index}] is invalid")
        key = str(entry.get("target_object_key") or "")
        if not key.startswith(f"v8-ab/{run_id}/recovered/"):
            raise ValueError(f"entries[{index}] is outside the run recovery prefix")
        digest = str(entry.get("source_sha256") or "")
        size = entry.get("source_size")
        if (
            not SHA256.fullmatch(digest)
            or isinstance(size, bool)
            or not isinstance(size, int)
            or size < 0
        ):
            raise ValueError(f"entries[{index}] hash/size is invalid")
        target = contained(fixture_root, key)
        if target in seen:
            raise ValueError("recovery plan contains duplicate object targets")
        seen.add(target)
        if target.exists() and (
            not target.is_file()
            or target.stat().st_size != size
            or sha256_file(target) != digest
        ):
            raise ValueError(f"refusing to delete drifted recovery object: {key}")
        targets.append((target, key, size, digest))
    removed = []
    for target, key, _, _ in targets:
        if target.exists():
            target.unlink()
            removed.append(key)
    return {
        "schema_version": 1,
        "status": "ROLLED_BACK",
        "run_id": run_id,
        "removed_object_keys": removed,
        "already_absent_count": len(targets) - len(removed),
        "database_write_performed": False,
        "production_write_performed": False,
    }


def atomic_write(path: pathlib.Path, value: Any) -> None:
    if path.exists():
        raise FileExistsError(f"refusing to overwrite rollback report: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    temporary = pathlib.Path(name)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(canonical_bytes(value))
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--plan", type=pathlib.Path, required=True)
    parser.add_argument("--fixture-root", type=pathlib.Path, required=True)
    parser.add_argument("--report", type=pathlib.Path, required=True)
    parser.add_argument("--execute", action="store_true")
    args = parser.parse_args()
    if not args.execute:
        raise ValueError("--execute is required")
    if args.report.exists() or args.report.is_symlink():
        raise FileExistsError("refusing to overwrite rollback report")
    if args.plan.is_symlink() or not args.plan.is_file():
        raise ValueError("--plan must be an existing non-symlink file")
    root = args.fixture_root
    if (
        not root.is_absolute()
        or not root.is_dir()
        or root.is_symlink()
        or root.name != "fixture-upload-b"
    ):
        raise ValueError("--fixture-root must be an absolute fixture-upload-b directory")
    plan = json.loads(args.plan.read_text(encoding="utf-8"))
    if not isinstance(plan, dict):
        raise ValueError("recovery plan must be an object")
    atomic_write(args.report, rollback(plan, root.resolve()))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
