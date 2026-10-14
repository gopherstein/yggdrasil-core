#!/usr/bin/env python3
"""Tests for packaging/linux/apt-signed-source.sh (#599)."""

import os
import pathlib
import subprocess
import tempfile
import unittest

SCRIPT = pathlib.Path(__file__).resolve().parent.parent / "packaging" / "linux" / "apt-signed-source.sh"
TRUSTED = "deb [trusted=yes] https://raw.githubusercontent.com/yeixio/toskar-core/apt stable main\n"


class AptSignedSource(unittest.TestCase):
    def setUp(self):
        self.dir = pathlib.Path(tempfile.mkdtemp())
        self.sources = self.dir / "sources.list.d"
        self.sources.mkdir()
        self.keyring = self.dir / "toskar-archive-keyring.gpg"

    def run_script(self):
        env = dict(os.environ, TOSKAR_APT_SOURCES=str(self.sources), TOSKAR_APT_KEYRING=str(self.keyring))
        subprocess.run(["sh", str(SCRIPT)], check=True, env=env, capture_output=True)

    def test_moves_a_trusted_source_to_the_signed_form(self):
        self.keyring.write_bytes(b"key")
        (self.sources / "yggdrasil.list").write_text(TRUSTED)
        self.run_script()
        self.assertFalse((self.sources / "yggdrasil.list").exists())
        text = (self.sources / "toskar.list").read_text()
        self.assertIn(f"deb [signed-by={self.keyring}] https://raw.githubusercontent.com/yeixio/toskar-core/apt stable main", text)
        self.assertNotIn("trusted=yes", text)

    def test_the_old_address_moves_too(self):
        self.keyring.write_bytes(b"key")
        (self.sources / "toskar.list").write_text(TRUSTED.replace("toskar-core", "yggdrasil-core"))
        self.run_script()
        self.assertIn("signed-by=", (self.sources / "toskar.list").read_text())

    def test_nothing_changes_without_a_key(self):
        (self.sources / "yggdrasil.list").write_text(TRUSTED)
        self.run_script()
        self.assertEqual((self.sources / "yggdrasil.list").read_text(), TRUSTED)
        self.assertFalse((self.sources / "toskar.list").exists())

    def test_a_signed_source_and_other_repositories_stay(self):
        self.keyring.write_bytes(b"key")
        signed = "deb [signed-by=/etc/apt/keyrings/toskar.gpg] https://raw.githubusercontent.com/yeixio/toskar-core/apt stable main\n"
        other = "deb [trusted=yes] https://example.com/repo stable main\n"
        (self.sources / "toskar.list").write_text(signed)
        (self.sources / "yggdrasil.list").write_text(other)
        self.run_script()
        self.assertEqual((self.sources / "toskar.list").read_text(), signed)
        self.assertEqual((self.sources / "yggdrasil.list").read_text(), other)


if __name__ == "__main__":
    unittest.main()
