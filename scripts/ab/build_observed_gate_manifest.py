#!/usr/bin/env python3
"""Build a hash-bound G5/G6 manifest for the Clone B hold-open coordinator."""

from __future__ import annotations

import argparse
import hashlib
import os
import pathlib
import re
import tempfile
from typing import Any

try:
    from scripts.ab import run_g4_clone_hold_open as hold
except ModuleNotFoundError:
    import run_g4_clone_hold_open as hold


SAFE_NAME = re.compile(r"[^A-Za-z0-9._-]+")


def ensure_inside(root: pathlib.Path, path: pathlib.Path, label: str) -> None:
    try:
        relative = path.relative_to(root)
    except ValueError:
        raise ValueError(f"{label} must be inside clone-root") from None
    current = root
    for part in relative.parts:
        current /= part
        if current.is_symlink():
            raise ValueError(f"{label} must not traverse a symlink")
    try:
        path.resolve().relative_to(root.resolve())
    except ValueError:
        raise ValueError(f"{label} must be inside clone-root") from None


def read_stable_bytes(path: pathlib.Path, label: str) -> bytes:
    return hold.read_regular_file_bytes(path, label)


def atomic_write_bytes(path: pathlib.Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.parent.is_symlink():
        raise ValueError("output parent must not be a symlink")
    with tempfile.NamedTemporaryFile(
        dir=path.parent, prefix=f".{path.name}.", delete=False
    ) as handle:
        temporary = pathlib.Path(handle.name)
        try:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        except BaseException:
            temporary.unlink(missing_ok=True)
            raise
    try:
        try:
            os.link(temporary, path)
        except FileExistsError:
            if read_stable_bytes(path, f"concurrent output {path}") != data:
                raise ValueError(
                    f"concurrent output differs; refusing to overwrite: {path}"
                ) from None
        else:
            directory_fd = os.open(
                path.parent,
                os.O_RDONLY | getattr(os, "O_DIRECTORY", 0),
            )
            try:
                os.fsync(directory_fd)
            finally:
                os.close(directory_fd)
    finally:
        temporary.unlink(missing_ok=True)


def copy_artifact(
    source: pathlib.Path,
    destination_dir: pathlib.Path,
    clone_root: pathlib.Path,
    ordinal: int,
    *,
    frozen_data: bytes | None = None,
) -> dict[str, Any]:
    data = (
        frozen_data
        if frozen_data is not None
        else read_stable_bytes(source, f"artifact {source}")
    )
    digest = hashlib.sha256(data).hexdigest()
    safe_name = SAFE_NAME.sub("_", source.name).strip("._") or "artifact"
    target = destination_dir / f"{ordinal:03d}-{digest}-{safe_name[:120]}"
    ensure_inside(clone_root, target, "artifact target")
    if target.exists():
        if target.is_symlink() or read_stable_bytes(target, "existing artifact") != data:
            raise ValueError(f"existing artifact differs: {target}")
    else:
        atomic_write_bytes(target, data)
    if (
        hashlib.sha256(
            read_stable_bytes(target, f"copied artifact {target}")
        ).hexdigest()
        != digest
    ):
        raise ValueError(f"artifact copy verification failed: {target}")
    return hold.relative_file_identity(target, clone_root)


def build(args: argparse.Namespace) -> dict[str, Any]:
    clone_root = args.clone_root
    if (
        not clone_root.is_absolute()
        or clone_root.is_symlink()
        or not clone_root.is_dir()
    ):
        raise ValueError("clone-root must be an absolute non-symlink directory")
    output = args.output
    if not output.is_absolute():
        raise ValueError("output must be absolute")
    ensure_inside(clone_root, output, "output")
    ensure_inside(clone_root, args.hold_open_ready, "HOLD_OPEN_READY")
    ready = hold.read_hashed_json(
        args.hold_open_ready, "HOLD_OPEN_READY ledger"
    )
    if (
        ready.get("kind") != "clone-b-hold-open-ready"
        or ready.get("status") != "HOLD_OPEN_READY"
        or ready.get("clone_side") != "B"
        or ready.get("database_host_class") != "local"
        or ready.get("process_groups_quiescent") is not True
        or ready.get("production_writes_executed") is not False
    ):
        raise ValueError("HOLD_OPEN_READY identity is invalid")

    result_data = read_stable_bytes(args.result_json, f"{args.gate} result")
    result_sha256 = hashlib.sha256(result_data).hexdigest()
    result = hold.read_strict_json_bytes(
        result_data, f"{args.gate} result"
    )
    status, violation_count = hold.normalize_observed_result(args.gate, result)
    sources = [args.result_json, *args.artifact]
    resolved_sources = [path.resolve() for path in sources]
    if len(set(resolved_sources)) != len(resolved_sources):
        raise ValueError("artifact inputs must be unique")
    destination_dir = clone_root / "observed" / args.gate.lower()
    ensure_inside(clone_root, destination_dir, "observed artifact directory")
    artifacts = []
    for ordinal, path in enumerate(sources, start=1):
        artifacts.append(
            copy_artifact(
                path,
                destination_dir,
                clone_root,
                ordinal,
                frozen_data=result_data if ordinal == 1 else None,
            )
        )
    if artifacts[0]["sha256"] != result_sha256:
        raise ValueError(f"{args.gate} result changed before it was copied")
    document = hold.add_self_hash(
        {
            "schema_version": hold.SCHEMA_VERSION,
            "gate": args.gate,
            "status": status,
            "violation_count": violation_count,
            "hold_open_ledger_sha256": ready[hold.DOCUMENT_HASH_FIELD],
            "artifacts": artifacts,
        },
        hold.EVIDENCE_HASH_FIELD,
    )
    encoded = hold.g4.canonical_bytes(document)
    if output.exists():
        if output.is_symlink() or read_stable_bytes(output, "existing output") != encoded:
            raise ValueError("existing output differs; refusing to overwrite")
    else:
        atomic_write_bytes(output, encoded)
    return {
        "status": "BUILT",
        "gate": args.gate,
        "gate_status": status,
        "violation_count": violation_count,
        "output": str(output),
        "sha256": hold.g4.sha256_file(output),
        "evidence_sha256": document[hold.EVIDENCE_HASH_FIELD],
        "artifact_count": len(artifacts),
    }


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--gate", choices=("G5", "G6"), required=True)
    parser.add_argument("--result-json", type=pathlib.Path, required=True)
    parser.add_argument("--hold-open-ready", type=pathlib.Path, required=True)
    parser.add_argument("--clone-root", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--artifact", type=pathlib.Path, action="append", default=[])
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    try:
        report = build(parse_args(argv))
        print(hold.g4.canonical_bytes(report).decode("utf-8"), end="")
        return 0
    except (OSError, UnicodeError, ValueError) as exc:
        print(str(exc), file=__import__("sys").stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
