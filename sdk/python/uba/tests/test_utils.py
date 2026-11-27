"""工具函数单测（与各端 SDK 的 utils 测试对齐）。"""

import re
import unittest
from datetime import datetime, timezone

from uba.utils import backoff_delay_ms, fnv1a32, merge_properties, session_in_sample, to_rfc3339_ms

UUID_PATTERN = re.compile(
    r"^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"
)


class TestUtils(unittest.TestCase):
    def test_to_rfc3339_ms(self):
        dt = datetime(2026, 3, 27, 12, 0, 0, 123000, tzinfo=timezone.utc)
        self.assertEqual(to_rfc3339_ms(dt), "2026-03-27T12:00:00.123Z")
        dt2 = datetime(2026, 1, 2, 3, 4, 5, 6000, tzinfo=timezone.utc)
        self.assertEqual(to_rfc3339_ms(dt2), "2026-01-02T03:04:05.006Z")

    def test_fnv1a32_deterministic(self):
        self.assertEqual(fnv1a32("session-a"), fnv1a32("session-a"))
        self.assertNotEqual(fnv1a32("session-a"), fnv1a32("session-b"))

    def test_session_in_sample(self):
        self.assertTrue(session_in_sample("s", 1.0))
        self.assertFalse(session_in_sample("s", 0.0))
        r = session_in_sample("fixed-session", 0.5)
        for _ in range(10):
            self.assertEqual(session_in_sample("fixed-session", 0.5), r)

    def test_merge_properties(self):
        base = {"a": "1", "b": "1"}
        merged = merge_properties(base, {"b": "2", "c": "3"})
        self.assertEqual(merged, {"a": "1", "b": "2", "c": "3"})
        merged["a"] = "mutated"
        self.assertEqual(base["a"], "1")  # 不污染原 dict
        self.assertIsNone(merge_properties(None, None))

    def test_backoff(self):
        self.assertEqual(backoff_delay_ms(1000, 0), 1000)
        self.assertEqual(backoff_delay_ms(1000, 1), 2000)
        self.assertEqual(backoff_delay_ms(1000, 2), 4000)


if __name__ == "__main__":
    unittest.main()
