"""Tests for release_train.py's release notes and weekly sync batching.

Run: python3 -m unittest discover -s scripts/ci -p 'test_*.py'
"""
import os
import sys
import unittest
from datetime import datetime, timezone

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from release_train import SYNC, release_notes_section, sync_release_due  # noqa: E402


def utc(s):
    return datetime.fromisoformat(s).replace(tzinfo=timezone.utc)


class ReleaseNotesSection(unittest.TestCase):
    def test_stops_at_the_next_heading(self):
        self.assertEqual(release_notes_section("## Release notes\n- a\n\n## Testing\nx"), "- a")

    def test_drops_the_attribution_footer(self):
        body = "## Release notes\n- a\n- b\n\n🤖 Generated with [Claude Code](https://claude.com/claude-code)\n"
        self.assertEqual(release_notes_section(body), "- a\n- b")

    def test_stops_at_a_rule(self):
        self.assertEqual(release_notes_section("## Release notes\n- a\n---\nfooter"), "- a")

    def test_missing_section(self):
        self.assertEqual(release_notes_section("no notes"), "")



class WeeklySyncRelease(unittest.TestCase):
    # 2026-11-19 is a Thursday.
    def test_any_releases_immediately(self):
        self.assertEqual(sync_release_due(utc("2026-11-18T12:00"), utc("2026-11-18T12:05"), "any"), (True, None))

    def test_holds_midweek_after_last_weeks_release(self):
        due, nxt = sync_release_due(utc("2026-11-12T14:05"), utc("2026-11-17T13:30"), "thu")
        self.assertFalse(due)
        self.assertEqual(nxt, utc("2026-11-19T13:00"))

    def test_releases_on_thursday_after_the_window_opens(self):
        self.assertTrue(sync_release_due(utc("2026-11-12T14:05"), utc("2026-11-19T14:02"), "thu")[0])

    def test_holds_on_thursday_before_the_window_opens(self):
        self.assertFalse(sync_release_due(utc("2026-11-12T14:05"), utc("2026-11-19T12:59"), "thu")[0])

    def test_late_check_still_releases_after_a_missed_thursday(self):
        # Thursday's live check failed; Friday's run must not wait a week.
        self.assertTrue(sync_release_due(utc("2026-11-12T14:05"), utc("2026-11-20T14:02"), "thu")[0])

    def test_once_per_window(self):
        self.assertFalse(sync_release_due(utc("2026-11-19T14:03"), utc("2026-11-19T20:00"), "thu")[0])

    def test_midweek_fix_release_does_not_skip_thursday(self):
        self.assertTrue(sync_release_due(utc("2026-11-17T18:00"), utc("2026-11-19T14:02"), "thu")[0])

    def test_sync_subjects(self):
        self.assertTrue(SYNC.match("chore: sync CLI to OpenAPI spec (#251)"))
        self.assertTrue(SYNC.match("feat: sync CLI to OpenAPI spec, with new commands (#223)"))
        self.assertFalse(SYNC.match("fix: require --text on follow-up turns (#241)"))


if __name__ == "__main__":
    unittest.main()
