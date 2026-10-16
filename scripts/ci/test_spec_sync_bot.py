"""Tests for the spec-sync bot's handling of untrusted upstream text.

Run: python3 -m unittest discover -s scripts/ci -p 'test_*.py'
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from spec_sync_bot import fenced, inert  # noqa: E402


class Inert(unittest.TestCase):
    def test_closing_keywords_and_references(self):
        text = inert("Fixes #12, closes owner/repo#3, resolved https://github.com/o/r/issues/4")
        for live in ("Fixes #", "closes owner", "resolved https", "#12", "#3"):
            self.assertNotIn(live, text)

    def test_mentions(self):
        self.assertNotIn("@org", inert("ping @org/team"))

    def test_html_comment_and_control_characters(self):
        text = inert("<!-- hide \x1b[31m‮")
        self.assertNotIn("<!--", text)
        self.assertNotIn("\x1b", text)
        self.assertNotIn("‮", text)

    def test_ordinary_text_is_untouched(self):
        for text in ("a fixed-width field", "closes the stream", "line one\nline two", "user@example.com"[:4]):
            self.assertEqual(inert(text), text)


class Fenced(unittest.TestCase):
    def test_text_cannot_close_the_fence(self):
        block = fenced("before\n````\nCloses #77")
        fence = block.split("\n", 1)[0]
        self.assertGreater(len(fence), 4)
        self.assertTrue(block.endswith("\n" + fence))


if __name__ == "__main__":
    unittest.main()
