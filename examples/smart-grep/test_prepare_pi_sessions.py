import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).with_name("prepare_pi_sessions.py")


class PrepareTests(unittest.TestCase):
    def run_preparer(self, root, *args):
        return subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(root), *args],
            capture_output=True,
            text=True,
        )

    def write_rows(self, root, name, rows):
        (Path(root) / name).write_text("\n".join(json.dumps(row) for row in rows) + "\n")

    def test_real_schema_counts_and_redacts(self):
        with tempfile.TemporaryDirectory() as directory:
            self.write_rows(Path(directory), "session.jsonl", [
                {"type": "message", "message": {"role": "toolResult", "toolName": "watcher_status", "isError": True, "content": [{"text": "stale PRIVATE_TOOL /secret/session"}]}},
                {"type": "message", "message": {"role": "assistant", "errorMessage": "quota PRIVATE_PROVIDER"}},
            ])
            result = self.run_preparer(directory)
            self.assertEqual(result.returncode, 0)
            self.assertIn("watcher.stale", result.stdout)
            self.assertIn("provider.quota", result.stdout)
            for secret in ("PRIVATE", "/secret", "session.jsonl"):
                self.assertNotIn(secret, result.stdout)

    def test_unknown_tool_name_is_fixed_and_redacted(self):
        with tempfile.TemporaryDirectory() as directory:
            self.write_rows(Path(directory), "session.jsonl", [{
                "type": "message", "message": {"role": "toolResult", "toolName": "bash-SECRET-url", "isError": True, "content": [{"text": "failure"}]},
            }])
            result = self.run_preparer(directory)
            self.assertEqual(result.returncode, 0)
            self.assertIn('"id":"tool.other"', result.stdout)
            self.assertNotIn("bash-SECRET-url", result.stdout)

    def test_nested_unrelated_error_is_ignored(self):
        with tempfile.TemporaryDirectory() as directory:
            self.write_rows(Path(directory), "session.jsonl", [{"type": "message", "message": {"role": "assistant", "content": [{"errorMessage": "PRIVATE"}]}}])
            result = self.run_preparer(directory)
            self.assertEqual(result.returncode, 0)
            self.assertNotIn("provider.", result.stdout)

    def test_deterministic_order_and_repeat_counts(self):
        with tempfile.TemporaryDirectory() as directory:
            row = {"type": "message", "message": {"role": "assistant", "errorMessage": "quota"}}
            self.write_rows(Path(directory), "a.jsonl", [row, row])
            self.write_rows(Path(directory), "b.jsonl", [{"type": "message", "message": {"role": "assistant", "errorMessage": "quota"}}])
            self.write_rows(Path(directory), "c.jsonl", [{"type": "message", "message": {"role": "toolResult", "toolName": "watcher_verify", "isError": True, "content": [{"text": "target ambiguous matches"}]}}])
            result = self.run_preparer(directory)
            self.assertEqual(result.returncode, 0)
            self.assertLess(result.stdout.index("provider.quota"), result.stdout.index("watcher.ambiguous"))
            self.assertIn("3 events across 2 session files, with 1 repeat events", result.stdout)

    def test_malformed_and_all_caps_have_no_partial_stdout(self):
        with tempfile.TemporaryDirectory() as directory:
            self.write_rows(Path(directory), "bad.jsonl", [{"type": "message"}])
            path = Path(directory) / "bad.jsonl"
            with path.open("a") as output:
                output.write("{bad\n")
            result = self.run_preparer(directory)
            self.assertEqual(result.returncode, 2)
            self.assertEqual(result.stdout, "")
            for option, value in (("--max-files", "0"), ("--max-bytes", "1"), ("--max-candidates", "0"), ("--max-payload-bytes", "1"), ("--days", "0"), ("--since", "2"),):
                args = (option, value, "--until", "1") if option == "--since" else (option, value)
                capped = self.run_preparer(directory, *args)
                self.assertEqual(capped.returncode, 2)
                self.assertEqual(capped.stdout, "")

    def test_until_and_old_files_are_excluded(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "old.jsonl"
            path.write_text(json.dumps({"type": "message", "message": {"role": "assistant", "errorMessage": "quota"}}))
            os.utime(path, (1, 1))
            result = self.run_preparer(directory, "--since", "2", "--until", "3")
            self.assertEqual(result.returncode, 0)
            self.assertIn('"id":"none"', result.stdout)


if __name__ == "__main__":
    unittest.main()
