import hashlib
import hmac
import json
import tempfile
import unittest
from pathlib import Path

import receiver


class ReceiverTest(unittest.TestCase):
    def test_wire_bytes_and_replay_window(self):
        raw = '{"title": "订单同步"}'.encode()
        key = b'word\x00\xff'
        signature = "sha256=" + hmac.new(key, b"1000." + raw, hashlib.sha256).hexdigest()
        for when in (700, 1000, 1300):
            self.assertTrue(receiver.valid_signature(key, "1000", signature, raw, when))
        for when in (699, 1301):
            self.assertFalse(receiver.valid_signature(key, "1000", signature, raw, when))
        self.assertFalse(receiver.valid_signature(key, "1000", signature, raw + b" ", 1000))
        self.assertFalse(receiver.valid_signature(key, "1000", "wrong", raw, 1000))

    def test_durable_lifecycle(self):
        with tempfile.TemporaryDirectory() as tmp:
            db = str(Path(tmp) / "events.db")
            receiver.initialize(db)
            def put(event, at, severity='warning', ident='incident'):
                body = json.dumps(dict(event=event, kind="future-kind", severity=severity, source="test",
                                       first_at="2026-01-01 00:00:00", id=ident, title="标题", message="详情", at=at)).encode()
                receiver.record(db, receiver.parse_event(body), body)
            for _ in range(2):
                put('firing', '2026-01-01 00:00:00')
                put('repeat', '2026-01-01 00:30:00')
            put('repeat', '2026-01-01 01:00:00')
            self.assertEqual(receiver.feed(db, True), {'data': {'active_count': 1, 'items': []}})
            put('resolved', '2026-01-01 01:05:00', 'info')
            put('repeat', '2026-01-01 00:45:00')
            put('firing', '2026-01-01 02:00:00', 'info', 'channel-test')
            page = receiver.feed(db, False)['data']
            self.assertEqual(page['active_count'], 0)
            self.assertEqual(len(page['items']), 6)
            self.assertEqual(page['items'][1]['event'], 'resolved')

if __name__ == "__main__":
    unittest.main()
