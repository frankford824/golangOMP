#!/usr/bin/env python3
"""Generate reversible API-session fixtures for two isolated UI clones.

The generator never connects to MySQL. It writes bearer headers with mode 0600,
hash-only SQL, cleanup SQL, and a secret-free self-hashed manifest. Operators
must independently verify the selected users and execute the SQL only against
the named local clone databases.
"""
from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import pathlib
import re
import secrets
import tempfile
import uuid
from dataclasses import dataclass
from typing import Any, Callable


SCHEMA_VERSION = 1
IDENTITY_RE = re.compile(r"^[a-z][a-z0-9_-]{1,31}$")
RUN_ID_RE = re.compile(r"^[a-z0-9][a-z0-9_-]{2,63}$")
DATABASE_RE = re.compile(r"^[A-Za-z0-9_]+$")
CLONE_A_RE = re.compile(r"^ab_[A-Za-z0-9_]*_a_ui(?:_[A-Za-z0-9]+)*$")
CLONE_B_RE = re.compile(r"^ab_[A-Za-z0-9_]*_b_ui(?:_[A-Za-z0-9]+)*$")
SESSION_NAMESPACE = uuid.UUID("9c4ee974-09da-4d83-8d1a-d3929acb3486")


@dataclass(frozen=True)
class Identity:
    identity_id: str
    user_id: int
    role: str


def canonical_json(value: Any) -> str:
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def parse_identity(value: str) -> Identity:
    parts = value.split(":", 2)
    if len(parts) != 3:
        raise argparse.ArgumentTypeError(
            "identity must be ID:USER_ID:ROLE"
        )
    identity_id, raw_user_id, role = parts
    if not IDENTITY_RE.fullmatch(identity_id):
        raise argparse.ArgumentTypeError("identity ID is invalid")
    try:
        user_id = int(raw_user_id)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("identity USER_ID is invalid") from exc
    if user_id <= 0 or not role.strip() or len(role) > 64:
        raise argparse.ArgumentTypeError("identity user/role is invalid")
    return Identity(identity_id, user_id, role.strip())


def quote_sql(value: str) -> str:
    if "\x00" in value or "\r" in value or "\n" in value:
        raise ValueError("SQL fixture value contains a forbidden character")
    return "'" + value.replace("\\", "\\\\").replace("'", "''") + "'"


def database_name(value: str, pattern: re.Pattern[str], label: str) -> str:
    if not DATABASE_RE.fullmatch(value) or not pattern.fullmatch(value):
        raise ValueError(
            f"{label} must name an isolated *_a_ui or *_b_ui clone, "
            "optionally followed by an alphanumeric run suffix"
        )
    return value


def sql_for_database(
    database: str,
    fixtures: list[dict[str, Any]],
    *,
    cleanup: bool,
) -> bytes:
    table = f"`{database}`.`user_sessions`"
    lines = [
        "SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;",
        "START TRANSACTION;",
    ]
    if cleanup:
        session_ids = ",".join(
            quote_sql(item["session_id"]) for item in fixtures
        )
        lines.append(
            f"DELETE FROM {table} WHERE session_id IN ({session_ids});"
        )
    else:
        for item in fixtures:
            lines.append(
                f"INSERT INTO {table} "
                "(session_id,user_id,token_hash,expires_at,last_seen_at,created_at) "
                f"SELECT {quote_sql(item['session_id'])},{item['user_id']},"
                f"{quote_sql(item['token_sha256'])},"
                f"{quote_sql(item['expires_at'])},UTC_TIMESTAMP(6),UTC_TIMESTAMP(6) "
                f"FROM `{database}`.`users` "
                f"WHERE id={item['user_id']} AND status='active';"
            )
    lines.extend(["COMMIT;", ""])
    return "\n".join(lines).encode("utf-8")


def verification_sql(database: str, fixtures: list[dict[str, Any]]) -> bytes:
    session_ids = ",".join(
        quote_sql(item["session_id"]) for item in fixtures
    )
    return (
        "SET SESSION TRANSACTION READ ONLY;\n"
        "START TRANSACTION READ ONLY;\n"
        "SELECT s.session_id,s.user_id,u.username,u.status,s.token_hash,"
        "s.expires_at\n"
        f"FROM `{database}`.`user_sessions` s\n"
        f"JOIN `{database}`.`users` u ON u.id=s.user_id\n"
        f"WHERE s.session_id IN ({session_ids})\n"
        "ORDER BY s.session_id;\n"
        "ROLLBACK;\n"
    ).encode("utf-8")


def atomic_write(path: pathlib.Path, payload: bytes, mode: int) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists():
        raise FileExistsError(f"refusing to overwrite fixture output: {path}")
    temporary: pathlib.Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            dir=path.parent,
            prefix=path.name + ".",
            suffix=".tmp",
            delete=False,
        ) as handle:
            temporary = pathlib.Path(handle.name)
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(temporary, mode)
        os.replace(temporary, path)
        temporary = None
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def build(
    *,
    run_id: str,
    clone_a_database: str,
    clone_b_database: str,
    identities: list[Identity],
    output_dir: pathlib.Path,
    now: dt.datetime,
    token_factory: Callable[[int], str] = secrets.token_urlsafe,
) -> dict[str, Any]:
    if not RUN_ID_RE.fullmatch(run_id):
        raise ValueError("run_id is invalid")
    clone_a_database = database_name(clone_a_database, CLONE_A_RE, "Clone A")
    clone_b_database = database_name(clone_b_database, CLONE_B_RE, "Clone B")
    if clone_a_database == clone_b_database:
        raise ValueError("Clone A and Clone B databases must be distinct")
    if not identities:
        raise ValueError("at least one identity is required")
    if len({item.identity_id for item in identities}) != len(identities):
        raise ValueError("identity IDs must be unique")
    if len({item.user_id for item in identities}) != len(identities):
        raise ValueError("identity user IDs must be unique")
    if now.tzinfo is None:
        raise ValueError("fixture timestamp must be timezone-aware")
    now = now.astimezone(dt.timezone.utc)
    expires_at = (now + dt.timedelta(hours=24)).strftime(
        "%Y-%m-%d %H:%M:%S.%f"
    )

    fixtures: list[dict[str, Any]] = []
    header_payloads: dict[str, bytes] = {}
    for identity in sorted(identities, key=lambda item: item.identity_id):
        token = token_factory(48)
        if not isinstance(token, str) or len(token) < 48:
            raise ValueError("token factory returned an insufficient token")
        token_sha = sha256_bytes(token.encode("utf-8"))
        session_id = "v8ab-" + str(
            uuid.uuid5(
                SESSION_NAMESPACE,
                f"{run_id}:{identity.identity_id}:{identity.user_id}",
            )
        )
        header_name = f"{identity.identity_id}.headers.json"
        header_payloads[header_name] = (
            canonical_json({"Authorization": "Bearer " + token}) + "\n"
        ).encode("utf-8")
        fixtures.append(
            {
                "identity_id": identity.identity_id,
                "role": identity.role,
                "user_id": identity.user_id,
                "session_id": session_id,
                "token_sha256": token_sha,
                "expires_at": expires_at,
                "headers_file": header_name,
            }
        )

    outputs: dict[str, tuple[bytes, int]] = {}
    for name, payload in header_payloads.items():
        outputs[name] = (payload, 0o600)
    outputs["insert-a.sql"] = (
        sql_for_database(clone_a_database, fixtures, cleanup=False),
        0o600,
    )
    outputs["insert-b.sql"] = (
        sql_for_database(clone_b_database, fixtures, cleanup=False),
        0o600,
    )
    outputs["cleanup-a.sql"] = (
        sql_for_database(clone_a_database, fixtures, cleanup=True),
        0o600,
    )
    outputs["cleanup-b.sql"] = (
        sql_for_database(clone_b_database, fixtures, cleanup=True),
        0o600,
    )
    outputs["verify-a.sql"] = (
        verification_sql(clone_a_database, fixtures),
        0o600,
    )
    outputs["verify-b.sql"] = (
        verification_sql(clone_b_database, fixtures),
        0o600,
    )

    manifest = {
        "schema_version": SCHEMA_VERSION,
        "status": "PREPARED",
        "run_id": run_id,
        "generated_at": now.isoformat().replace("+00:00", "Z"),
        "expires_at": expires_at + "Z",
        "clone_a_database": clone_a_database,
        "clone_b_database": clone_b_database,
        "identity_count": len(fixtures),
        "identities": fixtures,
        "artifacts": [
            {
                "path": name,
                "sha256": sha256_bytes(payload),
                "size": len(payload),
                "secret": name.endswith(".headers.json"),
            }
            for name, (payload, _mode) in sorted(outputs.items())
        ],
        "database_write_performed": False,
        "production_write_performed": False,
    }
    manifest["evidence_hash"] = sha256_bytes(
        canonical_json(manifest).encode("utf-8")
    )
    outputs["fixture-manifest.json"] = (
        (canonical_json(manifest) + "\n").encode("utf-8"),
        0o600,
    )
    if output_dir.exists():
        raise FileExistsError("fixture output directory already exists")
    output_dir.mkdir(parents=True)
    for name, (payload, mode) in outputs.items():
        atomic_write(output_dir / name, payload, mode)
    return manifest


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--clone-a-database", required=True)
    parser.add_argument("--clone-b-database", required=True)
    parser.add_argument(
        "--identity",
        action="append",
        type=parse_identity,
        required=True,
        help="repeat ID:USER_ID:ROLE",
    )
    parser.add_argument("--output-dir", type=pathlib.Path, required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    build(
        run_id=args.run_id,
        clone_a_database=args.clone_a_database,
        clone_b_database=args.clone_b_database,
        identities=args.identity,
        output_dir=args.output_dir,
        now=dt.datetime.now(dt.timezone.utc),
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
