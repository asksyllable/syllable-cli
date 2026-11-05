"""Tests for the spec-sync bot's handling of untrusted upstream text.

Run: python3 -m unittest discover -s scripts/ci -p 'test_*.py'
"""
import hashlib
import os
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import spec_sync_bot  # noqa: E402
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


class Followups(unittest.TestCase):
    def file(self, followups, existing=""):
        calls = []

        def fake_gh(*args, check=True):
            calls.append(args)
            return existing if args[:2] == ("issue", "list") else ""
        with mock.patch.object(spec_sync_bot, "gh", fake_gh):
            spec_sync_bot.file_followups(followups, pr="12")
        return [c for c in calls if c[:2] == ("issue", "create")]

    def test_each_follow_up_is_filed_once(self):
        old = f"<!-- cli-followup: {hashlib.sha256(b'already filed').hexdigest()[:16]} -->"
        created = self.file(["already filed", "`widgets create`: add a\nflag"], existing=old)
        self.assertEqual(len(created), 1)
        args = created[0]
        self.assertEqual(args[args.index("--title") + 1], "Follow-up: widgets create: add a flag")
        self.assertIn("in #12", args[args.index("--body") + 1])

    def test_nothing_to_file(self):
        self.assertEqual(self.file([]), [])


if __name__ == "__main__":
    unittest.main()
