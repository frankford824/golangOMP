"""Tailnet webhook and local-only OA notification feed. Uses Python's standard library."""
import base64
import hashlib
import hmac
import ipaddress
import json
import logging
import os
import re
import socketserver
import sqlite3
import threading
import time
from contextlib import closing
from datetime import datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

MAX_BODY = 65536
PATH = "/internal/dw-alerts"
SOCKET = "/run/oa-dw-alerts/feed.sock"
DATABASE = "/var/lib/oa-dw-alerts/events.sqlite3"


def valid_signature(key, stamp, signature, body, now):
    if not re.fullmatch(r"[0-9]{1,12}", stamp) or abs(int(now) - int(stamp)) > 300:
        return False
    if not re.fullmatch(r"sha256=[0-9a-f]{64}", signature):
        return False
    expected = hmac.new(key, stamp.encode("ascii") + b"." + body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature[7:])


def parse_event(body):
    event = json.loads(body.decode("utf-8"))
    if not isinstance(event, dict) or event.get("event") not in ("firing", "repeat", "resolved"):
        raise ValueError("event")
    if event.get("severity") not in ("info", "warning", "critical"):
        raise ValueError("severity")
    for field in ("source", "id", "kind", "title", "message", "first_at", "at"):
        if not isinstance(event.get(field), str) or not event[field].strip():
            raise ValueError(field)
    for field in ("first_at", "at"):
        datetime.strptime(event[field], "%Y-%m-%d %H:%M:%S")
    if not isinstance(event.get("details", {}), dict):
        raise ValueError("details")
    return event


def initialize(db):
    with closing(sqlite3.connect(db)) as conn:
        conn.execute("PRAGMA journal_mode=WAL")
        conn.executescript("""
            CREATE TABLE IF NOT EXISTS events (
              seq INTEGER PRIMARY KEY, source TEXT NOT NULL, id TEXT NOT NULL,
              event TEXT NOT NULL, occurrence TEXT NOT NULL, at TEXT NOT NULL,
              kind TEXT NOT NULL, severity TEXT NOT NULL, title TEXT NOT NULL,
              message TEXT NOT NULL, raw BLOB NOT NULL,
              UNIQUE(source,id,event,occurrence)
            );
            CREATE INDEX IF NOT EXISTS events_at ON events(at DESC,seq DESC);
        """)


def record(db, event, body):
    # A repeated delivery can have a fresh signature. Its event identity is stable.
    occurrence = event["at"] if event["event"] == "repeat" else ""
    with closing(sqlite3.connect(db, timeout=0.8)) as conn, conn:
        conn.execute("PRAGMA synchronous=FULL")
        conn.execute("""INSERT INTO events (source,id,event,occurrence,at,kind,severity,title,message,raw)
            VALUES (:source,:id,:event,:occurrence,:at,:kind,:severity,:title,:message,:raw)
            ON CONFLICT(source,id,event,occurrence) DO NOTHING""",
                     {**event, "occurrence": occurrence, "raw": body})


def feed(db, summary):
    with closing(sqlite3.connect(db, timeout=0.8)) as conn:
        conn.row_factory = sqlite3.Row
        conn.execute("BEGIN")
        count = conn.execute("""SELECT COUNT(*) FROM (
          SELECT source,id FROM events GROUP BY source,id
          HAVING MAX(event='resolved')=0 AND MAX(severity IN ('warning','critical'))=1
        )""").fetchone()[0]
        items = [] if summary else [dict(row) for row in conn.execute("""
          SELECT seq,source,id,event,at,kind,severity,title,message FROM events ORDER BY at DESC,seq DESC LIMIT 100""")]
    return {"data": {"active_count": count, "items": items}}


class Handler(BaseHTTPRequestHandler):
    timeout = 2

    def reply(self, status, body=b""):
        self.send_response(status)
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if body:
            self.wfile.write(body)

    def log_message(self, fmt, *args):
        pass


class Webhook(Handler):
    def do_POST(self):
        if self.path != PATH:
            return self.reply(404)
        if self.headers.get_content_type() != "application/json":
            return self.reply(415)
        length = self.headers.get("Content-Length", "")
        if not re.fullmatch(r"[0-9]{1,10}", length) or self.headers.get("Transfer-Encoding"):
            return self.reply(400)
        if int(length) > MAX_BODY:
            return self.reply(413)
        body = self.rfile.read(int(length))
        if len(body) != int(length):
            return self.reply(400)
        if not valid_signature(self.server.secret, self.headers.get("X-DW-Timestamp", ""),
                               self.headers.get("X-DW-Signature", ""), body, time.time()):
            return self.reply(401)
        try:
            event = parse_event(body)
            record(self.server.db, event, body)
        except (ValueError, RecursionError, UnicodeError):
            return self.reply(400)
        except sqlite3.Error:
            logging.exception("Unable to persist watchdog event")
            return self.reply(503)
        self.reply(204)


class Feed(Handler):
    def do_GET(self):
        if self.path not in ("/recent", "/summary"):
            return self.reply(404)
        try:
            payload = json.dumps(feed(self.server.db, self.path == "/summary"), ensure_ascii=False).encode()
        except sqlite3.Error:
            logging.exception("Unable to read watchdog feed")
            return self.reply(503)
        self.reply(200, payload)


class LocalServer(socketserver.ThreadingMixIn, socketserver.UnixStreamServer):
    daemon_threads = True


def main():
    address = ipaddress.IPv4Address(os.environ["DW_ALERT_ADDRESS"])
    if address not in ipaddress.IPv4Network("100.64.0.0/10"):
        raise ValueError("DW_ALERT_ADDRESS must be a tailnet IPv4 address")
    secret = base64.b64decode(os.environ.pop("DW_ALERT_SECRET_B64"), validate=True)
    if not secret:
        raise ValueError("missing webhook key")
    initialize(DATABASE)
    Path(SOCKET).unlink(missing_ok=True)
    with LocalServer(SOCKET, Feed) as local, ThreadingHTTPServer((str(address), 8090), Webhook) as web:
        os.chmod(SOCKET, 0o660)
        local.db = web.db = DATABASE
        web.secret = secret
        threading.Thread(target=local.serve_forever, daemon=True).start()
        web.serve_forever()


if __name__ == "__main__":
    main()
