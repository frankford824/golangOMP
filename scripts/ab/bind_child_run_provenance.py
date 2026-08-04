#!/usr/bin/env python3
"""Bind immutable child-run files to one formal V8 A/B run.

Large, read-only acquisition runs may be completed before the formal run is
created.  This tool preserves that separation without silently mixing run
roots: every regular file in every named child run is size/hash inventoried,
and the resulting document is itself content-addressed.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import re
from typing import Any


RUN_ID = re.compile(r"^[a-z0-9][a-z0-9_-]{2,63}$")
CHILD_NAME = re.compile(r"^[a-z][a-z0-9_-]{1,31}$")


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


def parse_child(value: str) -> tuple[str, pathlib.Path]:
    name, separator, raw_path = value.partition("=")
    if not separator or not CHILD_NAME.fullmatch(name):
        raise ValueError("--child must be NAME=/absolute/run/path")
    path = pathlib.Path(raw_path)
    if not path.is_absolute():
        raise ValueError(f"child run {name} path must be absolute")
    return name, path.resolve()


def inside_v8_ab(path: pathlib.Path) -> bool:
    parts = tuple(part.lower() for part in path.parts)
    return any(
        parts[index : index + 2] == ("tmp", "v8-ab")
        for index in range(len(parts) - 1)
    )


def inventory(path: pathlib.Path) -> tuple[list[dict[str, Any]], str]:
    if not path.is_dir() or not inside_v8_ab(path):
        raise ValueError(f"child run must be an existing tmp/v8-ab directory: {path}")
    rows: list[dict[str, Any]] = []
    for item in sorted(path.rglob("*")):
        if item.is_symlink():
            raise ValueError(f"child run contains a symlink: {item}")
        if item.is_file():
            relative = item.relative_to(path).as_posix()
            rows.append(
                {
                    "path": relative,
                    "size": item.stat().st_size,
                    "sha256": sha256_file(item),
                }
            )
        elif not item.is_dir():
            raise ValueError(f"child run contains a non-regular entry: {item}")
    if not rows:
        raise ValueError(f"child run contains no files: {path}")
    digest = hashlib.sha256(canonical_bytes(rows)).hexdigest()
    return rows, digest


def bind(
    formal_run_dir: pathlib.Path,
    children: list[tuple[str, pathlib.Path]],
) -> dict[str, Any]:
    formal = formal_run_dir.resolve()
    run_id = formal.name
    if (
        not formal.is_dir()
        or not inside_v8_ab(formal)
        or not RUN_ID.fullmatch(run_id)
    ):
        raise ValueError("formal run must be an existing tmp/v8-ab run directory")
    if not children:
        raise ValueError("at least one child run is required")
    names = [name for name, _ in children]
    paths = [path for _, path in children]
    if len(names) != len(set(names)) or len(paths) != len(set(paths)):
        raise ValueError("child run names and paths must be unique")
    if formal in paths:
        raise ValueError("formal run cannot bind itself as a child")

    records: list[dict[str, Any]] = []
    for name, path in sorted(children):
        files, tree_sha256 = inventory(path)
        records.append(
            {
                "name": name,
                "source_run_id": path.name,
                "source_run_path": str(path),
                "file_count": len(files),
                "byte_count": sum(row["size"] for row in files),
                "tree_sha256": tree_sha256,
                "files": files,
            }
        )
    result: dict[str, Any] = {
        "schema_version": 1,
        "status": "PASS",
        "violation_count": 0,
        "formal_run_id": run_id,
        "children": records,
    }
    result["binding_sha256"] = hashlib.sha256(canonical_bytes(result)).hexdigest()
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--formal-run-dir", required=True, type=pathlib.Path)
    parser.add_argument("--child", action="append", default=[])
    parser.add_argument("--output", required=True, type=pathlib.Path)
    args = parser.parse_args()
    try:
        children = [parse_child(value) for value in args.child]
        formal = args.formal_run_dir.resolve()
        output = args.output.resolve()
        if output.parent != formal or output.exists():
            raise ValueError("output must be a new direct child of the formal run")
        result = bind(formal, children)
        with output.open("x", encoding="utf-8", newline="\n") as handle:
            handle.write(canonical_bytes(result).decode("utf-8"))
    except (OSError, ValueError) as exc:
        print(f"ERROR: {exc}")
        return 2
    print(
        json.dumps(
            {
                "binding_sha256": result["binding_sha256"],
                "child_count": len(result["children"]),
                "status": result["status"],
            },
            sort_keys=True,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
