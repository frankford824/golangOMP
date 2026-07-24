#!/usr/bin/env python3
"""Generate an exact G06 checkpoint-failure retry authorization.

The generator is local and read-only with respect to manifests, checkpoints,
and reprobe evidence.  It authorizes only checkpointed
object_manifest.size_inconsistent/content_length_differs_from_stream records
that have two matching one-target PASS reprobes.
"""
from __future__ import annotations

import argparse
import hashlib
import os
import pathlib
import tempfile
from dataclasses import dataclass
from typing import Any, Sequence

try:
    from scripts.ab import hydrate_object_manifest as hydrator
    from scripts.ab import object_manifest_verifier as verifier
except ModuleNotFoundError:  # Direct execution from scripts/ab.
    import hydrate_object_manifest as hydrator
    import object_manifest_verifier as verifier


TARGET_CODE = "object_manifest.size_inconsistent"
TARGET_DETAIL = "content_length_differs_from_stream"


@dataclass(frozen=True)
class ReprobePair:
    evidence1: pathlib.Path
    artifact1: pathlib.Path
    evidence2: pathlib.Path
    artifact2: pathlib.Path


class DummyUploadAdapter:
    """Origin-free adapter used only for storage-adapter classification."""

    base_url = "g06-authorization-generator://local"
    use_head = False


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def relative_path(path: pathlib.Path, parent: pathlib.Path) -> str:
    return pathlib.Path(
        os.path.relpath(path.resolve(), parent.resolve())
    ).as_posix()


def read_checkpoint(
    checkpoint_path: pathlib.Path,
    input_sha: str,
) -> tuple[
    bytes,
    str,
    dict[str, dict[str, Any]],
    dict[str, dict[str, Any]],
]:
    if checkpoint_path.is_symlink() or not checkpoint_path.is_file():
        raise ValueError("stable checkpoint is missing or is a symlink")
    raw_bytes = checkpoint_path.read_bytes()
    raw = hydrator._load_json_without_duplicate_keys(checkpoint_path)
    if (
        not isinstance(raw, dict)
        or set(raw) != {
            "schema_version",
            "input_manifest_sha256",
            "adapter_fingerprints",
            "completed",
            "failed",
        }
        or raw.get("schema_version") != hydrator.CHECKPOINT_SCHEMA_VERSION
        or not isinstance(raw.get("adapter_fingerprints"), dict)
    ):
        raise ValueError("checkpoint field contract differs")
    completed, failed = hydrator.load_checkpoint(
        checkpoint_path,
        input_sha,
        raw["adapter_fingerprints"],
    )
    return raw_bytes, sha256_bytes(raw_bytes), completed, failed


def artifact_target(
    artifact_path: pathlib.Path,
    config: verifier.VerifierConfig,
) -> tuple[str, tuple[int, str, str]]:
    if artifact_path.is_symlink() or not artifact_path.is_file():
        raise ValueError("reprobe artifact is missing or is a symlink")
    lines = artifact_path.read_text(encoding="utf-8").splitlines()
    if len(lines) != 1:
        raise ValueError("reprobe artifact must contain exactly one row")
    row = hydrator._load_json_without_duplicate_keys(artifact_path)
    hydrator.validate_hydration_row(row, 1)
    if not row["sha256"]:
        raise ValueError("reprobe artifact is not hydrated")
    resolved = hydrator.adapter_kind(config, row["storage_adapter"])
    if resolved is None:
        raise ValueError("reprobe artifact adapter is not configured")
    kind, _adapter = resolved
    return (
        hydrator.checkpoint_key(kind, row["object_key"]),
        (
            row["size"],
            verifier.normalize_mime(row["mime_type"]),
            row["sha256"],
        ),
    )


def reprobe_item(
    evidence_path: pathlib.Path,
    artifact_path: pathlib.Path,
    output_parent: pathlib.Path,
) -> dict[str, str]:
    if evidence_path.is_symlink() or not evidence_path.is_file():
        raise ValueError("reprobe evidence is missing or is a symlink")
    return {
        "evidence_path": relative_path(evidence_path, output_parent),
        "evidence_sha256": verifier.sha256_file(evidence_path),
        "artifact_path": relative_path(artifact_path, output_parent),
        "artifact_sha256": verifier.sha256_file(artifact_path),
    }


def generate_authorization(
    input_manifest_path: pathlib.Path,
    checkpoint_path: pathlib.Path,
    output_path: pathlib.Path,
    pairs: Sequence[ReprobePair],
) -> dict[str, Any]:
    if input_manifest_path.is_symlink() or not input_manifest_path.is_file():
        raise ValueError("input manifest is missing or is a symlink")
    if output_path.is_symlink():
        raise ValueError("output path must not be a symlink")
    output_resolved = output_path.resolve()
    if output_resolved in {
        input_manifest_path.resolve(),
        checkpoint_path.resolve(),
    }:
        raise ValueError("output path must differ from input and checkpoint")
    input_bytes = input_manifest_path.read_bytes()
    input_sha = sha256_bytes(input_bytes)
    checkpoint_bytes, checkpoint_sha, _completed, failed = read_checkpoint(
        checkpoint_path, input_sha
    )
    target_failed = {
        key: record
        for key, record in failed.items()
        if (
            record["violation_code"] == TARGET_CODE
            and record["detail"] == TARGET_DETAIL
        )
    }
    if not target_failed:
        raise ValueError("checkpoint contains no target size-inconsistent failures")
    if len(pairs) != len(target_failed):
        raise ValueError(
            "reprobe pair count does not cover all target checkpoint failures"
        )

    output_parent = output_path.resolve().parent
    output_parent.mkdir(parents=True, exist_ok=True)
    config = verifier.VerifierConfig(upload=DummyUploadAdapter())
    used_paths: set[pathlib.Path] = set()
    entries_by_key: dict[str, dict[str, Any]] = {}
    for pair in pairs:
        paths = tuple(
            path.resolve()
            for path in (
                pair.evidence1,
                pair.artifact1,
                pair.evidence2,
                pair.artifact2,
            )
        )
        if output_resolved in paths:
            raise ValueError("output path must differ from reprobe paths")
        if len(set(paths)) != 4 or used_paths.intersection(paths):
            raise ValueError("reprobe pair paths are duplicated")
        used_paths.update(paths)
        key1, metadata1 = artifact_target(pair.artifact1, config)
        key2, metadata2 = artifact_target(pair.artifact2, config)
        if key1 != key2 or metadata1 != metadata2:
            raise ValueError("reprobe pair artifacts do not agree")
        if key1 not in target_failed:
            raise ValueError("reprobe pair targets a non-authorizable failure")
        if key1 in entries_by_key:
            raise ValueError("checkpoint failure has duplicate reprobe pairs")
        record = target_failed[key1]
        entries_by_key[key1] = {
            "failure_record_sha256": (
                hydrator.checkpoint_failure_record_sha256(record)
            ),
            "reprobes": [
                reprobe_item(
                    pair.evidence1, pair.artifact1, output_parent
                ),
                reprobe_item(
                    pair.evidence2, pair.artifact2, output_parent
                ),
            ],
        }

    missing = set(target_failed) - set(entries_by_key)
    if missing:
        raise ValueError("target checkpoint failure is missing a reprobe pair")
    entries = sorted(
        entries_by_key.values(),
        key=lambda item: item["failure_record_sha256"],
    )
    payload = {
        "schema_version": (
            hydrator.FAILURE_RETRY_AUTHORIZATION_SCHEMA_VERSION
        ),
        "authorization_type": hydrator.FAILURE_RETRY_AUTHORIZATION_TYPE,
        "input_manifest_sha256": input_sha,
        "checkpoint_sha256": checkpoint_sha,
        "failure_retries": entries,
    }
    authorization_sha = sha256_bytes(
        verifier.canonical_json(payload).encode("utf-8")
    )
    authorization = dict(payload)
    authorization["authorization_sha256"] = authorization_sha

    temporary: pathlib.Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="wb",
            prefix=f".{output_path.name}.",
            suffix=".tmp",
            dir=output_parent,
            delete=False,
        ) as handle:
            temporary = pathlib.Path(handle.name)
            handle.write(
                (verifier.canonical_json(authorization) + "\n").encode("utf-8")
            )
            handle.flush()
            os.fsync(handle.fileno())
        authorized, validated_sha = (
            hydrator.load_failure_retry_authorization(
                temporary,
                input_sha=input_sha,
                checkpoint_sha=checkpoint_sha,
                failed_targets=failed,
                config=config,
            )
        )
        if authorized != set(target_failed) or validated_sha != authorization_sha:
            raise ValueError("generated authorization self-validation differs")
        if input_manifest_path.read_bytes() != input_bytes:
            raise ValueError("input manifest changed during authorization generation")
        if checkpoint_path.read_bytes() != checkpoint_bytes:
            raise ValueError("checkpoint changed during authorization generation")
        os.replace(temporary, output_path)
        temporary = None
    finally:
        if temporary is not None:
            try:
                temporary.unlink()
            except FileNotFoundError:
                pass
    return authorization


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input_manifest", type=pathlib.Path)
    parser.add_argument("checkpoint", type=pathlib.Path)
    parser.add_argument("output", type=pathlib.Path)
    parser.add_argument(
        "--reprobe-pair",
        action="append",
        nargs=4,
        required=True,
        metavar=("EVIDENCE1", "ARTIFACT1", "EVIDENCE2", "ARTIFACT2"),
        help="repeat once for each target checkpoint failure",
    )
    return parser.parse_args(argv)


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_args(argv)
    pairs = [
        ReprobePair(*(pathlib.Path(value) for value in values))
        for values in args.reprobe_pair
    ]
    generate_authorization(
        args.input_manifest,
        args.checkpoint,
        args.output,
        pairs,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
