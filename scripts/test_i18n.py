#!/usr/bin/env python3
"""Tests for scripts/i18n.py."""

import importlib.util
import json
import pathlib
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("i18n", pathlib.Path(__file__).with_name("i18n.py"))
i18n = importlib.util.module_from_spec(spec)
spec.loader.exec_module(i18n)


def catalog(d: pathlib.Path, language: str, namespace: str, data: dict) -> None:
    folder = d / "locales" / language
    folder.mkdir(parents=True, exist_ok=True)
    (folder / f"{namespace}.json").write_text(json.dumps(data), encoding="utf-8")


class StatusTest(unittest.TestCase):
    def test_missing_and_same_as_english(self):
        english = {
            "chat:a": "Retry",
            "chat:b": "Chat",
            "chat:count": "{{count}}",
            "chat:items_one": "{{count}} item",
            "chat:items_other": "{{count}} items",
            "chat:gone": "Not translated yet",
        }
        japanese = {"chat:a": "再試行", "chat:b": "Chat", "chat:count": "{{count}}", "chat:items_other": "{{count}}件"}
        missing, same = i18n.status(english, japanese)
        # Japanese has only _other, which is all it needs.
        self.assertEqual(missing, ["chat:gone"])
        # A placeholder alone is nothing to translate.
        self.assertEqual(same, ["chat:b"])

    def test_glossary_reads_the_catalog(self):
        with tempfile.TemporaryDirectory() as tmp:
            d = pathlib.Path(tmp)
            catalog(d, "en", "common", {"nav": {"models": "Models"}})
            catalog(d, "de", "common", {"nav": {"models": "Modelle"}})
            (d / "glossary.json").write_text(json.dumps({"terms": [{"term": "Models", "key": "common:nav.models"}]}))
            self.assertEqual(i18n.glossary("de", d), [("Models", "Models", "Modelle")])
            self.assertEqual(i18n.check(d), [])

    def test_check_finds_a_bad_glossary(self):
        with tempfile.TemporaryDirectory() as tmp:
            d = pathlib.Path(tmp)
            catalog(d, "en", "common", {"nav": {"models": "Models"}})
            terms = [{"term": "Models", "key": "common:nav.models"}, {"term": "Models", "key": "common:nav.gone"}]
            (d / "glossary.json").write_text(json.dumps({"terms": terms}))
            problems = i18n.check(d)
        self.assertEqual(len(problems), 2)
        self.assertIn("common:nav.gone", problems[0])
        self.assertIn("listed twice", problems[1])

    def test_the_real_glossary(self):
        self.assertEqual(i18n.check(), [])


if __name__ == "__main__":
    unittest.main()
