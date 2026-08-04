#!/usr/bin/env python3
"""Fetch and fingerprint automatically adjudicated source-bundle members.

The task-asset membership, object keys, sizes, MIME types, and storage
references are frozen before network access. The read-only SSH/OSS protocol
then streams each object into a run-scoped local seed directory while learning
the previously absent SHA-256. No remote write is possible.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import struct
from typing import Any

try:
    from scripts.ab import fetch_asset_recovery_sources as controlled
    from scripts.ab import prepare_auto_source_bundle_manifest as prepare
except ModuleNotFoundError:
    import fetch_asset_recovery_sources as controlled
    import prepare_auto_source_bundle_manifest as prepare


SOURCE_DIRECTORY = "frozen-upload-seed-b"


def canonical_bytes(value: Any) -> bytes:
    return (
        json.dumps(
            value,
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        )
        + "\n"
    ).encode("utf-8")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(controlled.CHUNK_BYTES), b""):
            digest.update(chunk)
    return digest.hexdigest()


def safe_target(root: pathlib.Path, object_key: str) -> pathlib.Path:
    relative = pathlib.PurePosixPath(object_key)
    if (
        relative.is_absolute()
        or not relative.parts
        or "\\" in object_key
        or "\x00" in object_key
        or any(part in {"", ".", ".."} for part in relative.parts)
    ):
        raise ValueError("bundle source contains an unsafe object key")
    target = root.joinpath(*relative.parts)
    current = root
    for part in relative.parts[:-1]:
        current = current / part
        if current.is_symlink():
            raise ValueError(
                "symlinked bundle source target is forbidden"
            )
        current.mkdir(exist_ok=True)
    resolved_parent = target.parent.resolve(strict=True)
    try:
        resolved_parent.relative_to(root)
    except ValueError:
        raise ValueError("bundle source target escaped run root") from None
    if target.is_symlink():
        raise ValueError("symlinked bundle source target is forbidden")
    return target


def mime_compatible(expected_mime: str, actual_mime: str) -> bool:
    expected = controlled.normalize_mime(expected_mime)
    actual = controlled.normalize_mime(actual_mime)
    return actual == expected or (
        actual == "application/octet-stream" and bool(expected)
    )


def request_header(
    adapter: controlled.SSHControlledReadAdapter,
    *,
    object_key: str,
    size: int,
    mime_type: str,
) -> tuple[dict[str, Any], str]:
    adapter.ensure_process()
    assert adapter.stdin is not None and adapter.stdout is not None
    request = controlled.canonical_bytes(
        {"max_object_bytes": size, "object_key": object_key}
    )
    adapter.stdin.write(struct.pack("!I", len(request)) + request)
    adapter.stdin.flush()
    header = controlled.read_frame(adapter.stdout)
    actual_mime = controlled.normalize_mime(
        str(header.get("mime") or "")
    )
    expected_mime = controlled.normalize_mime(mime_type)
    if (
        set(header) != {"detail", "mime", "size", "status"}
        or header.get("status") != 200
        or header.get("detail") != ""
        or header.get("size") != size
        or not mime_compatible(expected_mime, actual_mime)
    ):
        try:
            adapter.stdin.write(b"\x00")
            adapter.stdin.flush()
        except OSError:
            adapter.broken = True
        raise controlled.ControlledReadError(
            "remote metadata differs from frozen automatic-bundle allowlist: "
            f"object_key={object_key!r}, "
            f"expected_size={size}, actual_size={header.get('size')!r}, "
            f"expected_mime={expected_mime!r}, "
            f"actual_mime={actual_mime!r}, "
            f"status={header.get('status')!r}, detail={header.get('detail')!r}"
        )
    return header, actual_mime


def probe_metadata(
    adapter: controlled.SSHControlledReadAdapter,
    *,
    object_key: str,
    size: int,
    mime_type: str,
) -> str:
    _, actual_mime = request_header(
        adapter,
        object_key=object_key,
        size=size,
        mime_type=mime_type,
    )
    assert adapter.stdin is not None
    adapter.stdin.write(b"\x00")
    adapter.stdin.flush()
    return actual_mime


def fetch_unpinned(
    adapter: controlled.SSHControlledReadAdapter,
    *,
    object_key: str,
    size: int,
    mime_type: str,
    target: pathlib.Path,
) -> tuple[str, str]:
    _, actual_mime = request_header(
        adapter,
        object_key=object_key,
        size=size,
        mime_type=mime_type,
    )
    assert adapter.stdin is not None and adapter.stdout is not None
    adapter.stdin.write(b"\x01")
    adapter.stdin.flush()
    digest = hashlib.sha256()
    temporary = target.with_name(f".{target.name}.partial")
    if temporary.exists():
        temporary.unlink()
    try:
        with temporary.open("xb") as handle:
            remaining = size
            while remaining:
                chunk = controlled.read_exact(
                    adapter.stdout,
                    min(controlled.CHUNK_BYTES, remaining),
                )
                handle.write(chunk)
                digest.update(chunk)
                remaining -= len(chunk)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, target)
    finally:
        temporary.unlink(missing_ok=True)
    if target.stat().st_size != size:
        raise controlled.ControlledReadError(
            "downloaded automatic-bundle object size drifted"
        )
    return digest.hexdigest(), actual_mime


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mapping", type=pathlib.Path, required=True)
    parser.add_argument("--object-manifest", type=pathlib.Path, required=True)
    parser.add_argument("--run-root", type=pathlib.Path, required=True)
    parser.add_argument("--ssh-host", required=True)
    parser.add_argument("--ssh-env-file", required=True)
    parser.add_argument("--timeout-seconds", type=float, default=3600)
    parser.add_argument("--hydrated-output", type=pathlib.Path, required=True)
    parser.add_argument("--receipt", type=pathlib.Path, required=True)
    args = parser.parse_args()
    if args.hydrated_output.exists() or args.receipt.exists():
        raise FileExistsError("refusing to overwrite automatic-bundle outputs")
    run_root = args.run_root.resolve(strict=True)
    if not run_root.is_dir() or run_root.is_symlink():
        raise ValueError("run-root must be an existing real directory")
    source_root = run_root / SOURCE_DIRECTORY
    source_root.mkdir(mode=0o700, exist_ok=True)
    if source_root.is_symlink():
        raise ValueError("source root must not be a symlink")
    source_root = source_root.resolve(strict=True)

    mapping = prepare.read_json_object(args.mapping, "mapping")
    bundle_rows = prepare.bundle_rows(mapping)
    member_ids = [
        member_id
        for row in bundle_rows
        for member_id in row["member_ids"]
    ]
    if len(member_ids) != len(set(member_ids)):
        raise ValueError("automatic bundle candidates reuse a task asset")
    object_rows = prepare.read_object_manifest(args.object_manifest)
    frozen = []
    for task_asset_id in member_ids:
        row = object_rows.get(task_asset_id)
        if (
            row is None
            or row.get("owner_kind") != "task_asset"
            or int(row.get("owner_id") or 0) != task_asset_id
            or row.get("status") not in {"active", "recorded"}
            or bool(row.get("is_placeholder"))
            or int(row.get("size") or 0) <= 0
            or not str(row.get("object_key") or "")
            or not str(row.get("mime_type") or "")
        ):
            raise ValueError(
                f"task asset {task_asset_id} lacks a frozen readable object"
            )
        frozen.append(dict(row))

    adapter = controlled.SSHControlledReadAdapter(
        args.ssh_host,
        args.ssh_env_file,
        args.timeout_seconds,
    )
    total_bytes = 0
    reused = 0
    try:
        origin_fingerprint = adapter.origin_fingerprint()
        for row in frozen:
            target = safe_target(source_root, str(row["object_key"]))
            expected_size = int(row["size"])
            if target.is_file() and target.stat().st_size == expected_size:
                digest = sha256_file(target)
                actual_mime = probe_metadata(
                    adapter,
                    object_key=str(row["object_key"]),
                    size=expected_size,
                    mime_type=str(row["mime_type"]),
                )
                reused += 1
            else:
                if target.exists():
                    raise ValueError(
                        f"existing source target is not reusable: {target}"
                    )
                digest, actual_mime = fetch_unpinned(
                    adapter,
                    object_key=str(row["object_key"]),
                    size=expected_size,
                    mime_type=str(row["mime_type"]),
                    target=target,
                )
            existing_sha = str(row.get("sha256") or "")
            if existing_sha and existing_sha != digest:
                raise ValueError(
                    f"task asset {row['owner_id']} differs from frozen SHA-256"
                )
            row["sha256"] = digest
            row["mime_type_from_manifest"] = str(row["mime_type"])
            row["mime_type_from_object"] = actual_mime
            row["mime_type"] = actual_mime
            total_bytes += expected_size
    finally:
        adapter.close()

    args.hydrated_output.parent.mkdir(parents=True, exist_ok=True)
    with args.hydrated_output.open("xb") as handle:
        for row in frozen:
            handle.write(canonical_bytes(row))
    receipt = {
        "schema_version": 1,
        "status": "PASS",
        "remote_operation": "GET",
        "remote_write_performed": False,
        "mapping_sha256": prepare.sha256_file(args.mapping),
        "input_object_manifest_sha256": prepare.sha256_file(
            args.object_manifest
        ),
        "hydrated_manifest_sha256": prepare.sha256_file(
            args.hydrated_output
        ),
        "origin_fingerprint_sha256": origin_fingerprint,
        "member_count": len(frozen),
        "reused_local_count": reused,
        "total_bytes": total_bytes,
        "source_root": str(source_root),
    }
    args.receipt.write_bytes(canonical_bytes(receipt))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
