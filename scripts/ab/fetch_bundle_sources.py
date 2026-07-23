#!/usr/bin/env python3
"""Fetch the frozen 7-scope/22-member bundle source allowlist by OSS GET.

The scope is pinned twice: the complete candidate and hydrated-manifest file
hashes are frozen below, and the exact ordered task/scope/revision membership is
also frozen. Runtime source rows are constructed only by an exact cross-join of
those two immutable inputs. No database connection or remote mutation exists.

Downloads use the controlled read-only SSH/OSS protocol shared with
``fetch_asset_recovery_sources.py``. Bytes remain in a private staging
directory until all 22 objects pass MIME, size, and SHA-256 validation and the
transport closes cleanly. Publication and its receipt are rollback-cleaned on
any failure. An exact existing receipt makes reruns network-free.
"""

from __future__ import annotations

import argparse
import datetime
import hashlib
import json
import os
import pathlib
import shutil
import sys
import tempfile
from dataclasses import dataclass
from typing import Any, Callable

try:
    from scripts.ab import fetch_asset_recovery_sources as controlled
except ModuleNotFoundError:
    import fetch_asset_recovery_sources as controlled


SCHEMA_VERSION = 1
RECEIPT_NAME = "controlled-bundle-source-receipts.json"
SOURCE_DIRECTORY = "frozen-upload-seed-b"
STAGING_PREFIX = ".bundle-source-staging."
FROZEN_CANDIDATE_FILE_SHA256 = (
    "5ec8bd6bb3cd43b718858bee84388d570028fc8ed6bde038f49e3a3b15f15bca"
)
FROZEN_CANDIDATE_DIGEST = (
    "ec5e55413da1dda70d493bf6c3e5cc0904d4be3a2768f7ff710fba210c0f73db"
)
FROZEN_HYDRATED_MANIFEST_SHA256 = (
    "6e7c366f704e6c1f004ee2864db83e067c3573bab342bb0186299e346b42c557"
)
FROZEN_SCOPE_ORDER = (
    (485, "sku", 365, 1, (293, 297)),
    (523, "sku", 398, 1, (402, 403, 404, 405)),
    (523, "sku", 400, 1, (358, 359, 360, 361)),
    (2234, "sku", 2401, 2, (12672, 12673)),
    (2251, "sku", 2417, 2, (13103, 13104, 13105, 13106, 13107)),
    (2477, "sku", 2725, 2, (18989, 18991, 18993)),
    (2598, "sku", 2869, 2, (20799, 20802)),
)
FROZEN_MEMBER_COUNT = 22


@dataclass(frozen=True)
class BundleSource:
    task_id: int
    scope_kind: str
    scope_ref_id: int
    revision_no: int
    task_asset_id: int
    asset_id: int
    storage_ref_id: str
    object_key: str
    size: int
    mime_type: str
    sha256: str
    original_file_name: str
    source_stage: str
    evidence_event_ids: tuple[str, ...]


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(controlled.CHUNK_BYTES), b""):
            digest.update(chunk)
    return digest.hexdigest()


def validated_regular_file(
    path: pathlib.Path, expected_sha256: str, label: str
) -> pathlib.Path:
    if not path.is_absolute() or not path.is_file() or path.is_symlink():
        raise ValueError(f"{label} must be an absolute regular non-symlink file")
    resolved = path.resolve(strict=True)
    if sha256_file(resolved) != expected_sha256:
        raise ValueError(f"{label} SHA-256 differs from the frozen input")
    return resolved


def read_json_lines(path: pathlib.Path) -> list[dict[str, Any]]:
    rows = []
    with path.open("r", encoding="utf-8") as handle:
        for line_number, raw in enumerate(handle, 1):
            if not raw.strip():
                raise ValueError(
                    f"hydrated manifest contains blank line {line_number}"
                )
            value = json.loads(raw)
            if not isinstance(value, dict):
                raise ValueError(
                    f"hydrated manifest line {line_number} is not an object"
                )
            rows.append(value)
    return rows


def safe_object_key(value: str) -> pathlib.PurePosixPath:
    relative = pathlib.PurePosixPath(value)
    if (
        not value
        or relative.is_absolute()
        or "\\" in value
        or "\x00" in value
        or any(part in {"", ".", ".."} for part in relative.parts)
    ):
        raise ValueError("frozen bundle source contains an unsafe object key")
    return relative


def normalize_mime(value: str) -> str:
    return value.split(";", 1)[0].strip().lower()


def build_frozen_sources(
    candidate: dict[str, Any],
    hydrated_rows: list[dict[str, Any]],
) -> tuple[BundleSource, ...]:
    if (
        candidate.get("schema_version") != 1
        or candidate.get("status") != "PROPOSED_REVIEW"
        or candidate.get("bundle_count") != len(FROZEN_SCOPE_ORDER)
        or candidate.get("member_count") != FROZEN_MEMBER_COUNT
        or candidate.get("source_candidate_sha256")
        != FROZEN_CANDIDATE_DIGEST
    ):
        raise ValueError("bundle candidate header differs from the frozen contract")
    bundles = candidate.get("bundles")
    if not isinstance(bundles, list) or len(bundles) != len(FROZEN_SCOPE_ORDER):
        raise ValueError("bundle candidate scope count differs from the allowlist")
    if len(hydrated_rows) != FROZEN_MEMBER_COUNT:
        raise ValueError("hydrated manifest row count differs from the allowlist")

    hydrated_by_id: dict[int, dict[str, Any]] = {}
    for row in hydrated_rows:
        entity_key = str(row.get("entity_key") or "")
        if not entity_key.startswith("task_asset:"):
            raise ValueError("hydrated manifest contains a non-task-asset row")
        try:
            task_asset_id = int(entity_key.removeprefix("task_asset:"))
        except ValueError:
            raise ValueError("hydrated manifest entity key is invalid") from None
        if task_asset_id <= 0 or task_asset_id in hydrated_by_id:
            raise ValueError("hydrated manifest task asset ids are invalid")
        hydrated_by_id[task_asset_id] = row

    sources: list[BundleSource] = []
    candidate_ids: list[int] = []
    for index, expected in enumerate(FROZEN_SCOPE_ORDER):
        task_id, scope_kind, scope_ref_id, revision_no, ordered_ids = expected
        bundle = bundles[index]
        if (
            not isinstance(bundle, dict)
            or bundle.get("task_id") != task_id
            or bundle.get("scope_kind") != scope_kind
            or bundle.get("scope_ref_id") != scope_ref_id
            or bundle.get("revision_no") != revision_no
            or bundle.get("confidence") != "proposed_review"
            or bundle.get("requires_human_member_confirmation") is not True
            or bundle.get("all_members_exist_and_hash_verified") is not True
        ):
            raise ValueError(
                f"bundle candidate scope {index} differs from the exact allowlist"
            )
        members = bundle.get("ordered_members")
        if (
            not isinstance(members, list)
            or tuple(member.get("task_asset_id") for member in members)
            != ordered_ids
        ):
            raise ValueError(
                f"bundle candidate scope {index} member order drifted"
            )
        for member in members:
            task_asset_id = member["task_asset_id"]
            hydrated = hydrated_by_id.get(task_asset_id)
            if hydrated is None:
                raise ValueError(
                    f"task asset {task_asset_id} is absent from hydrated manifest"
                )
            mime_type = normalize_mime(
                str(member.get("mime_type_from_object") or "")
            )
            object_key = str(member.get("object_key") or "")
            safe_object_key(object_key)
            expected_pairs = {
                "task_id": task_id,
                "storage_ref_id": member.get("storage_ref_id"),
                "object_key": object_key,
                "size": member.get("size"),
                "sha256": member.get("sha256"),
                "status": member.get("object_status"),
            }
            actual_pairs = {
                "task_id": hydrated.get("task_id"),
                "storage_ref_id": hydrated.get("storage_ref_id"),
                "object_key": hydrated.get("object_key"),
                "size": hydrated.get("size"),
                "sha256": hydrated.get("sha256"),
                "status": hydrated.get("status"),
            }
            evidence = member.get("evidence_event_ids")
            if (
                expected_pairs != actual_pairs
                or hydrated.get("owner_kind") != "task_asset"
                or hydrated.get("owner_id") != task_asset_id
                or hydrated.get("entity_key") != f"task_asset:{task_asset_id}"
                or member.get("task_id") != task_id
                or not isinstance(member.get("asset_id"), int)
                or isinstance(member.get("asset_id"), bool)
                or member["asset_id"] <= 0
                or member.get("asset_type") != "source"
                or member.get("upload_status") != "uploaded"
                or member.get("confirmed") is not False
                or mime_type == ""
                or normalize_mime(str(hydrated.get("mime_type") or ""))
                != mime_type
                or not controlled.SHA256.fullmatch(
                    str(member.get("sha256") or "")
                )
                or not isinstance(member.get("size"), int)
                or isinstance(member.get("size"), bool)
                or member["size"] <= 0
                or not isinstance(evidence, list)
                or not evidence
                or any(
                    not isinstance(event_id, str)
                    or not event_id.startswith(
                        ("task_event_log:", "task_module_event:")
                    )
                    for event_id in evidence
                )
            ):
                raise ValueError(
                    f"task asset {task_asset_id} candidate/hydrated evidence drifted"
                )
            sources.append(
                BundleSource(
                    task_id=task_id,
                    scope_kind=scope_kind,
                    scope_ref_id=scope_ref_id,
                    revision_no=revision_no,
                    task_asset_id=task_asset_id,
                    asset_id=int(member["asset_id"]),
                    storage_ref_id=str(member["storage_ref_id"]),
                    object_key=object_key,
                    size=int(member["size"]),
                    mime_type=mime_type,
                    sha256=str(member["sha256"]),
                    original_file_name=str(
                        member.get("original_file_name") or ""
                    ),
                    source_stage=str(member.get("source_stage") or ""),
                    evidence_event_ids=tuple(evidence),
                )
            )
            candidate_ids.append(task_asset_id)

    frozen_ids = [
        task_asset_id
        for _, _, _, _, task_asset_ids in FROZEN_SCOPE_ORDER
        for task_asset_id in task_asset_ids
    ]
    if (
        len(sources) != FROZEN_MEMBER_COUNT
        or candidate_ids != frozen_ids
        or set(hydrated_by_id) != set(frozen_ids)
    ):
        raise ValueError("bundle source set differs from the exact 22-member allowlist")
    return tuple(sources)


def load_frozen_sources(
    candidate_path: pathlib.Path,
    hydrated_manifest_path: pathlib.Path,
) -> tuple[tuple[BundleSource, ...], dict[str, str]]:
    candidate_path = validated_regular_file(
        candidate_path,
        FROZEN_CANDIDATE_FILE_SHA256,
        "candidate",
    )
    hydrated_manifest_path = validated_regular_file(
        hydrated_manifest_path,
        FROZEN_HYDRATED_MANIFEST_SHA256,
        "hydrated manifest",
    )
    candidate = json.loads(candidate_path.read_text(encoding="utf-8"))
    if not isinstance(candidate, dict):
        raise ValueError("candidate must be a JSON object")
    sources = build_frozen_sources(
        candidate,
        read_json_lines(hydrated_manifest_path),
    )
    return sources, {
        "candidate_file_sha256": FROZEN_CANDIDATE_FILE_SHA256,
        "candidate_digest": FROZEN_CANDIDATE_DIGEST,
        "hydrated_manifest_sha256": FROZEN_HYDRATED_MANIFEST_SHA256,
    }


def validated_run_root(value: pathlib.Path) -> pathlib.Path:
    return controlled.validated_run_root(value)


def contained_source_directory(run_root: pathlib.Path) -> pathlib.Path:
    candidate = run_root / SOURCE_DIRECTORY
    if candidate.exists():
        resolved = candidate.resolve(strict=True)
        if (
            not candidate.is_dir()
            or candidate.is_symlink()
            or run_root not in resolved.parents
        ):
            raise ValueError(
                f"{SOURCE_DIRECTORY} must be a contained real directory"
            )
        return resolved
    candidate.mkdir(mode=0o700)
    resolved = candidate.resolve(strict=True)
    if run_root not in resolved.parents:
        raise ValueError(f"{SOURCE_DIRECTORY} escaped run-root")
    return resolved


def target_path(source_dir: pathlib.Path, source: BundleSource) -> pathlib.Path:
    target = source_dir.joinpath(*safe_object_key(source.object_key).parts)
    resolved_parent = target.parent.resolve(strict=False)
    try:
        resolved_parent.relative_to(source_dir)
    except ValueError:
        raise ValueError("bundle source target escaped source root") from None
    if target.is_symlink():
        raise ValueError("symlinked bundle source target is forbidden")
    return target


def receipt_row(
    source: BundleSource,
    target: pathlib.Path,
    fetched_at: str,
) -> dict[str, Any]:
    return {
        "task_id": source.task_id,
        "scope_kind": source.scope_kind,
        "scope_ref_id": source.scope_ref_id,
        "revision_no": source.revision_no,
        "task_asset_id": source.task_asset_id,
        "asset_id": source.asset_id,
        "storage_ref_id": source.storage_ref_id,
        "object_key": source.object_key,
        "size": source.size,
        "mime_type": source.mime_type,
        "sha256": source.sha256,
        "source_local_path": str(target),
        "original_file_name": source.original_file_name,
        "source_stage": source.source_stage,
        "evidence_event_ids": list(source.evidence_event_ids),
        "fetched_at": fetched_at,
    }


def validate_existing_receipt(
    receipt_path: pathlib.Path,
    source_dir: pathlib.Path,
    sources: tuple[BundleSource, ...],
    input_hashes: dict[str, str],
) -> dict[str, Any] | None:
    if not receipt_path.exists():
        return None
    if not receipt_path.is_file() or receipt_path.is_symlink():
        raise ValueError("bundle source receipt must be a regular non-symlink file")
    evidence = json.loads(receipt_path.read_text(encoding="utf-8"))
    expected_keys = {
        "version",
        "status",
        "protocol",
        "remote_operation",
        "production_writes_executed",
        "database_connections_opened",
        "origin_fingerprint_sha256",
        "candidate_file_sha256",
        "candidate_digest",
        "hydrated_manifest_sha256",
        "bundle_count",
        "member_count",
        "total_bytes",
        "sources",
        "evidence_sha256",
    }
    if (
        not isinstance(evidence, dict)
        or set(evidence) != expected_keys
        or evidence.get("version") != SCHEMA_VERSION
        or evidence.get("status") != "PASS"
        or evidence.get("protocol") != controlled.PROTOCOL
        or evidence.get("remote_operation") != "GET"
        or evidence.get("production_writes_executed") is not False
        or evidence.get("database_connections_opened") is not False
        or evidence.get("candidate_file_sha256")
        != input_hashes["candidate_file_sha256"]
        or evidence.get("candidate_digest") != input_hashes["candidate_digest"]
        or evidence.get("hydrated_manifest_sha256")
        != input_hashes["hydrated_manifest_sha256"]
        or evidence.get("bundle_count") != len(FROZEN_SCOPE_ORDER)
        or evidence.get("member_count") != len(sources)
        or evidence.get("total_bytes") != sum(source.size for source in sources)
        or not controlled.SHA256.fullmatch(
            str(evidence.get("origin_fingerprint_sha256") or "")
        )
        or not isinstance(evidence.get("sources"), list)
        or len(evidence["sources"]) != len(sources)
    ):
        raise ValueError("existing bundle source receipt is invalid")
    unsigned = dict(evidence)
    signature = unsigned.pop("evidence_sha256")
    if (
        not isinstance(signature, str)
        or hashlib.sha256(canonical_bytes(unsigned)).hexdigest() != signature
    ):
        raise ValueError("existing bundle source receipt hash drifted")
    for source, row in zip(sources, evidence["sources"]):
        target = target_path(source_dir, source)
        if (
            not isinstance(row, dict)
            or row
            != receipt_row(
                source,
                target,
                str(row.get("fetched_at") or ""),
            )
            or not str(row.get("fetched_at") or "").strip()
            or not target.is_file()
            or target.is_symlink()
            or target.stat().st_size != source.size
            or sha256_file(target) != source.sha256
        ):
            raise ValueError("existing bundle source bytes or receipt drifted")
    return evidence


def remove_owned_staging(staging: pathlib.Path, run_root: pathlib.Path) -> None:
    resolved = staging.resolve(strict=False)
    if (
        resolved.parent != run_root
        or not resolved.name.startswith(STAGING_PREFIX)
    ):
        raise ValueError("refusing to clean an unowned staging directory")
    if resolved.exists():
        shutil.rmtree(resolved)


def run(
    args: argparse.Namespace,
    *,
    adapter_factory: Callable[[str, str, float], Any] = (
        controlled.SSHControlledReadAdapter
    ),
    now: Callable[[], datetime.datetime] = lambda: datetime.datetime.now(
        datetime.timezone.utc
    ),
) -> dict[str, Any]:
    run_root = validated_run_root(args.run_root)
    sources, input_hashes = load_frozen_sources(
        args.candidate,
        args.hydrated_manifest,
    )
    source_dir = contained_source_directory(run_root)
    receipt_path = source_dir / RECEIPT_NAME
    existing = validate_existing_receipt(
        receipt_path, source_dir, sources, input_hashes
    )
    if existing is not None:
        return existing
    existing_targets = [
        target_path(source_dir, source)
        for source in sources
        if target_path(source_dir, source).exists()
    ]
    if existing_targets:
        raise FileExistsError(
            "bundle source files exist without their exact controlled-read receipt"
        )

    staging = pathlib.Path(
        tempfile.mkdtemp(prefix=STAGING_PREFIX, dir=run_root)
    ).resolve()
    adapter = adapter_factory(
        args.ssh_host, args.ssh_env_file, args.timeout_seconds
    )
    adapter_closed = False
    published: list[pathlib.Path] = []
    try:
        origin_fingerprint = adapter.origin_fingerprint()
        if not controlled.SHA256.fullmatch(origin_fingerprint):
            raise controlled.ControlledReadError(
                "invalid controlled read origin fingerprint"
            )
        staged: list[tuple[BundleSource, pathlib.Path]] = []
        for index, source in enumerate(sources):
            temporary = staging / f"{index:03d}-{source.task_asset_id}.object"
            adapter.fetch_to_path(source, temporary)
            if (
                not temporary.is_file()
                or temporary.is_symlink()
                or temporary.stat().st_size != source.size
                or sha256_file(temporary) != source.sha256
            ):
                raise controlled.ControlledReadError(
                    "post-fetch bytes differ from frozen bundle allowlist"
                )
            staged.append((source, temporary))
        adapter.close()
        adapter_closed = True

        fetched_at = (
            now()
            .astimezone(datetime.timezone.utc)
            .isoformat()
            .replace("+00:00", "Z")
        )
        rows = []
        for source, temporary in staged:
            target = target_path(source_dir, source)
            target.parent.mkdir(parents=True, exist_ok=True)
            if target.exists():
                raise FileExistsError(
                    "bundle source target appeared during controlled read"
                )
            os.replace(temporary, target)
            published.append(target)
            rows.append(receipt_row(source, target, fetched_at))
        evidence = {
            "version": SCHEMA_VERSION,
            "status": "PASS",
            "protocol": controlled.PROTOCOL,
            "remote_operation": "GET",
            "production_writes_executed": False,
            "database_connections_opened": False,
            "origin_fingerprint_sha256": origin_fingerprint,
            **input_hashes,
            "bundle_count": len(FROZEN_SCOPE_ORDER),
            "member_count": len(sources),
            "total_bytes": sum(source.size for source in sources),
            "sources": rows,
        }
        evidence["evidence_sha256"] = hashlib.sha256(
            canonical_bytes(evidence)
        ).hexdigest()
        controlled.atomic_write(
            receipt_path, canonical_bytes(evidence) + b"\n"
        )
        return evidence
    except Exception:
        for path in reversed(published):
            try:
                path.unlink()
            except FileNotFoundError:
                pass
        try:
            receipt_path.unlink()
        except FileNotFoundError:
            pass
        raise
    finally:
        if not adapter_closed:
            try:
                adapter.close()
            except OSError:
                pass
        remove_owned_staging(staging, run_root)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-root", required=True, type=pathlib.Path)
    parser.add_argument("--candidate", required=True, type=pathlib.Path)
    parser.add_argument(
        "--hydrated-manifest", required=True, type=pathlib.Path
    )
    parser.add_argument("--ssh-host", required=True)
    parser.add_argument("--ssh-env-file", required=True)
    parser.add_argument("--timeout-seconds", type=float, default=3600.0)
    args = parser.parse_args(sys.argv[1:] if argv is None else argv)
    try:
        args.ssh_host = controlled.validate_ssh_host(args.ssh_host)
        args.ssh_env_file = controlled.validate_remote_env_path(
            args.ssh_env_file
        )
    except ValueError as exc:
        parser.error(str(exc))
    if args.timeout_seconds <= 0 or args.timeout_seconds > 3600:
        parser.error("--timeout-seconds must be in (0, 3600]")
    return args


def main() -> int:
    try:
        run(parse_args())
    except (
        controlled.ControlledReadError,
        FileExistsError,
        OSError,
        ValueError,
        json.JSONDecodeError,
    ) as exc:
        print(f"BLOCKED: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
