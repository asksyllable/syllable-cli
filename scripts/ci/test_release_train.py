"""Tests for release_train.py's release-notes extraction.

Run: python3 -m unittest discover -s scripts/ci -p 'test_*.py'
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from release_train import release_notes_section  # noqa: E402


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


if __name__ == "__main__":
    unittest.main()
