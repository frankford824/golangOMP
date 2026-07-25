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
EXACT_MISSING_IDS = {23989, 23990, 23991}
STAGE_TOKEN_XATTR = "user.codex_v8_stage_token"


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


def verify_self_bound(value: dict[str, Any], label: str) -> None:
    expected = str(value.get("evidence_sha256") or "")
    unsigned = dict(value)
    unsigned.pop("evidence_sha256", None)
    actual = hashlib.sha256(
        json.dumps(
            unsigned,
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        ).encode("utf-8")
    ).hexdigest()
    if not SHA256.fullmatch(expected) or actual != expected:
        raise ValueError(f"{label} self hash is missing or stale")


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


def fsync_directory(path: pathlib.Path) -> None:
    if os.name == "nt":
        return
    descriptor = os.open(
        path,
        os.O_RDONLY | getattr(os, "O_DIRECTORY", 0),
    )
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def fsync_directory_chain(
    leaf: pathlib.Path,
    root: pathlib.Path,
) -> None:
    current = leaf.resolve()
    boundary = root.resolve()
    try:
        current.relative_to(boundary)
    except ValueError:
        raise ValueError("durability path escapes the fixture root") from None
    while True:
        fsync_directory(current)
        if current == boundary:
            break
        current = current.parent


def durable_unlink(path: pathlib.Path) -> None:
    parent = path.parent
    path.unlink()
    fsync_directory(parent)


def cleanup_reserved_private(
    private: pathlib.Path,
    token: str,
    fixture_root: pathlib.Path,
) -> str | None:
    try:
        private.resolve(strict=False).relative_to(
            fixture_root.parent.resolve()
        )
    except ValueError:
        raise ValueError("reserved private path escapes the run root") from None
    if not private.is_absolute() or private.is_symlink() or not private.exists():
        return None
    if not private.is_file() or os.name == "nt":
        raise ValueError("reserved private ownership cannot be proven")
    try:
        actual = os.getxattr(private, STAGE_TOKEN_XATTR).decode("ascii")
    except (OSError, UnicodeError):
        raise ValueError("reserved private ownership cannot be proven") from None
    if not SHA256.fullmatch(token) or actual != token:
        raise ValueError("reserved private ownership cannot be proven")
    durable_unlink(private)
    return str(private)


def staging_is_owned(
    run_id: str,
    staging: pathlib.Path,
    receipt_path: pathlib.Path,
    fixture_root: pathlib.Path,
    expected_size: int,
    expected_sha256: str,
) -> bool:
    try:
        receipt_path.resolve(strict=False).relative_to(
            fixture_root.parent.resolve()
        )
    except ValueError:
        raise ValueError("staging ownership receipt escapes the run root") from None
    if (
        not receipt_path.is_absolute()
        or receipt_path.is_symlink()
        or not receipt_path.is_file()
    ):
        return False
    receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
    if not isinstance(receipt, dict):
        raise ValueError("staging ownership receipt must be an object")
    verify_self_bound(receipt, "recovery staging ownership receipt")
    stat = staging.stat()
    return (
        receipt.get("schema_version") == 1
        and receipt.get("status") == "STAGING_OWNED"
        and receipt.get("run_id") == run_id
        and receipt.get("staging_path") == str(staging.resolve())
        and receipt.get("device") == stat.st_dev
        and receipt.get("inode") == stat.st_ino
        and receipt.get("size") == expected_size == stat.st_size
        and receipt.get("sha256") == expected_sha256 == sha256_file(staging)
    )


def cleanup_owned_private_staging(
    run_id: str,
    staging: pathlib.Path,
    receipt_path: pathlib.Path,
    fixture_root: pathlib.Path,
    expected_size: int,
    expected_sha256: str,
) -> str | None:
    if not receipt_path.is_file() or receipt_path.is_symlink():
        return None
    receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
    if not isinstance(receipt, dict):
        raise ValueError("staging ownership receipt must be an object")
    verify_self_bound(receipt, "recovery staging ownership receipt")
    private_raw = receipt.get("private_path")
    if not isinstance(private_raw, str) or not private_raw:
        return None
    private = pathlib.Path(private_raw)
    try:
        private.resolve(strict=False).relative_to(
            fixture_root.parent.resolve()
        )
    except ValueError:
        raise ValueError(
            "private staging path escapes the run root"
        ) from None
    if (
        not private.is_absolute()
        or private.is_symlink()
        or private.resolve(strict=False)
        == staging.resolve(strict=False)
        or not private.exists()
    ):
        return None
    if not private.is_file():
        raise ValueError("private staging target is not a file")
    stat = private.stat()
    if (
        receipt.get("schema_version") != 1
        or receipt.get("status") != "STAGING_OWNED"
        or receipt.get("run_id") != run_id
        or receipt.get("device") != stat.st_dev
        or receipt.get("inode") != stat.st_ino
        or receipt.get("size") != expected_size
        or stat.st_size != expected_size
        or receipt.get("sha256") != expected_sha256
        or sha256_file(private) != expected_sha256
    ):
        raise ValueError("private staging ownership cannot be proven")
    durable_unlink(private)
    return str(private)


def rollback(plan: dict[str, Any], fixture_root: pathlib.Path) -> dict[str, Any]:
    expected_self_hash = str(plan.get("evidence_sha256") or "")
    unsigned = dict(plan)
    unsigned.pop("evidence_sha256", None)
    if (
        plan.get("version") != 1
        or plan.get("status") not in {"PREPARED", "MATERIALIZED"}
        or plan.get("database_writes_executed") is not False
        or plan.get("production_writes_executed") is not False
        or not SHA256.fullmatch(expected_self_hash)
        or hashlib.sha256(
            json.dumps(
                unsigned,
                ensure_ascii=False,
                sort_keys=True,
                separators=(",", ":"),
            ).encode("utf-8")
        ).hexdigest()
        != expected_self_hash
    ):
        raise ValueError("recovery plan is not an exact file-only materialization")
    run_id = str(plan.get("run_id") or "")
    if not RUN_ID.fullmatch(run_id):
        raise ValueError("recovery plan run_id is invalid")
    entries = plan.get("entries")
    if (
        not isinstance(entries, list)
        or len(entries) != len(EXACT_MISSING_IDS)
        or {
            entry.get("missing_task_asset_id")
            for entry in entries
            if isinstance(entry, dict)
        }
        != EXACT_MISSING_IDS
    ):
        raise ValueError("recovery plan entries are missing")
    targets: list[
        tuple[
            pathlib.Path,
            str,
            int,
            str,
            str,
            pathlib.Path,
            pathlib.Path,
            pathlib.Path,
            pathlib.Path,
            str,
        ]
    ] = []
    seen: set[pathlib.Path] = set()
    for index, entry in enumerate(entries):
        if not isinstance(entry, dict):
            raise ValueError(f"entries[{index}] is invalid")
        key = str(entry.get("target_object_key") or "")
        if not key.startswith(f"v8-ab/{run_id}/recovered/"):
            raise ValueError(f"entries[{index}] is outside the run recovery prefix")
        digest = str(entry.get("source_sha256") or "")
        size = entry.get("source_size")
        disposition = entry.get("rollback_registry", {}).get(
            "fixture_disposition"
        )
        staging = pathlib.Path(
            str(
                entry.get("rollback_registry", {}).get(
                    "staging_local_path"
                )
                or ""
            )
        )
        ownership_receipt = pathlib.Path(
            str(
                entry.get("rollback_registry", {}).get(
                    "ownership_receipt_path"
                )
                or ""
            )
        )
        staging_ownership_receipt = pathlib.Path(
            str(
                entry.get("rollback_registry", {}).get(
                    "staging_ownership_receipt_path"
                )
                or ""
            )
        )
        staging_private = pathlib.Path(
            str(
                entry.get("rollback_registry", {}).get(
                    "staging_private_path"
                )
                or ""
            )
        )
        staging_token = str(
            entry.get("rollback_registry", {}).get(
                "staging_ownership_token"
            )
            or ""
        )
        if (
            not SHA256.fullmatch(digest)
            or isinstance(size, bool)
            or not isinstance(size, int)
            or size < 0
            or disposition not in {"created", "reused_identical"}
        ):
            raise ValueError(f"entries[{index}] hash/size is invalid")
        try:
            staging.resolve(strict=False).relative_to(
                fixture_root.parent.resolve()
            )
        except ValueError:
            raise ValueError(
                f"entries[{index}] staging path escapes the run root"
            ) from None
        if not staging.is_absolute() or staging.is_symlink():
            raise ValueError(f"entries[{index}] staging path is invalid")
        try:
            ownership_receipt.resolve(strict=False).relative_to(
                fixture_root.parent.resolve()
            )
        except ValueError:
            raise ValueError(
                f"entries[{index}] ownership receipt escapes the run root"
            ) from None
        if (
            not ownership_receipt.is_absolute()
            or ownership_receipt.is_symlink()
        ):
            raise ValueError(
                f"entries[{index}] ownership receipt path is invalid"
            )
        try:
            staging_ownership_receipt.resolve(strict=False).relative_to(
                fixture_root.parent.resolve()
            )
        except ValueError:
            raise ValueError(
                f"entries[{index}] staging ownership receipt escapes the run root"
            ) from None
        if (
            not staging_ownership_receipt.is_absolute()
            or staging_ownership_receipt.is_symlink()
        ):
            raise ValueError(
                f"entries[{index}] staging ownership receipt path is invalid"
            )
        try:
            staging_private.resolve(strict=False).relative_to(
                fixture_root.parent.resolve()
            )
        except ValueError:
            raise ValueError(
                f"entries[{index}] private staging path escapes the run root"
            ) from None
        if (
            not staging_private.is_absolute()
            or staging_private.is_symlink()
            or not SHA256.fullmatch(staging_token)
        ):
            raise ValueError(
                f"entries[{index}] private staging reservation is invalid"
            )
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
        if disposition == "reused_identical" and not target.exists():
            raise ValueError(
                f"reused recovery object is missing: {key}"
            )
        targets.append(
            (
                target,
                key,
                size,
                digest,
                disposition,
                staging,
                ownership_receipt,
                staging_ownership_receipt,
                staging_private,
                staging_token,
            )
        )
    removed = []
    retained_reused = []
    removed_staging = []
    removed_private_staging = []
    already_absent = 0
    for (
        target,
        key,
        size,
        digest,
        disposition,
        staging,
        ownership_receipt,
        staging_ownership_receipt,
        staging_private,
        staging_token,
    ) in targets:
        removed_private = cleanup_owned_private_staging(
            run_id,
            staging,
            staging_ownership_receipt,
            fixture_root,
            size,
            digest,
        )
        if removed_private is not None:
            removed_private_staging.append(removed_private)
        reserved_private = cleanup_reserved_private(
            staging_private,
            staging_token,
            fixture_root,
        )
        if reserved_private is not None:
            removed_private_staging.append(reserved_private)
        if disposition == "created" and target.exists():
            owned = False
            if ownership_receipt.exists():
                if not ownership_receipt.is_file():
                    raise ValueError(
                        f"ownership receipt is not a file: {ownership_receipt}"
                    )
                try:
                    receipt = json.loads(
                        ownership_receipt.read_text(encoding="utf-8")
                    )
                except (OSError, UnicodeError, json.JSONDecodeError) as exc:
                    raise ValueError(
                        f"ownership receipt is unreadable: {ownership_receipt}"
                    ) from exc
                if not isinstance(receipt, dict):
                    raise ValueError("ownership receipt must be an object")
                verify_self_bound(receipt, "recovery ownership receipt")
                stat = target.stat()
                owned = (
                    receipt.get("schema_version") == 1
                    and receipt.get("status") == "OWNED_LINK"
                    and receipt.get("run_id") == run_id
                    and receipt.get("target_path") == str(target.resolve())
                    and receipt.get("staging_path") == str(staging.resolve())
                    and receipt.get("device") == stat.st_dev
                    and receipt.get("inode") == stat.st_ino
                    and receipt.get("size") == size == stat.st_size
                    and receipt.get("sha256") == digest
                )
            if (
                not owned
                and staging.exists()
                and staging.is_file()
                and not staging.is_symlink()
                and staging_is_owned(
                    run_id,
                    staging,
                    staging_ownership_receipt,
                    fixture_root,
                    size,
                    digest,
                )
                and os.path.samestat(staging.stat(), target.stat())
            ):
                owned = True
            if not owned:
                raise ValueError(
                    f"recovery target ownership cannot be proven: {key}"
                )
            durable_unlink(target)
            removed.append(key)
        elif disposition == "created":
            already_absent += 1
        elif disposition == "reused_identical":
            retained_reused.append(key)
        if staging.exists():
            if not staging.is_file() or staging.is_symlink():
                raise ValueError(
                    f"recovery staging target is unsafe: {staging}"
                )
            if not staging_is_owned(
                run_id,
                staging,
                staging_ownership_receipt,
                fixture_root,
                size,
                digest,
            ):
                raise ValueError(
                    f"recovery staging ownership cannot be proven: {staging}"
                )
            durable_unlink(staging)
            removed_staging.append(str(staging))
    object_root = (fixture_root / "objects").resolve()
    pruned_directories = []
    for target, _, _, _, disposition, _, _, _, _, _ in sorted(
        targets, key=lambda item: len(item[0].parts), reverse=True
    ):
        if disposition != "created":
            continue
        parent = target.parent
        while parent != object_root and object_root in parent.parents:
            try:
                parent.rmdir()
            except OSError:
                break
            fsync_directory_chain(parent.parent, object_root)
            pruned_directories.append(
                parent.relative_to(fixture_root).as_posix()
            )
            parent = parent.parent
    return {
        "schema_version": 1,
        "status": "ROLLED_BACK",
        "run_id": run_id,
        "removed_object_keys": removed,
        "already_absent_count": already_absent,
        "retained_reused_object_keys": retained_reused,
        "removed_staging_paths": removed_staging,
        "removed_private_staging_paths": removed_private_staging,
        "pruned_empty_directories": sorted(set(pruned_directories)),
        "database_write_performed": False,
        "production_write_performed": False,
    }


def atomic_write(path: pathlib.Path, value: Any) -> None:
    if path.exists() or path.is_symlink():
        raise FileExistsError(f"refusing to overwrite rollback report: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    temporary = pathlib.Path(name)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(canonical_bytes(value))
            handle.flush()
            os.fsync(handle.fileno())
        try:
            os.link(temporary, path)
        except FileExistsError:
            raise FileExistsError(
                f"refusing to overwrite racing rollback report: {path}"
            ) from None
        if os.name != "nt":
            directory = os.open(
                path.parent,
                os.O_RDONLY | getattr(os, "O_DIRECTORY", 0),
            )
            try:
                os.fsync(directory)
            finally:
                os.close(directory)
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
