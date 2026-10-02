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
            (d / "languages.json").write_text(json.dumps([{"code": "en"}, {"code": "de"}]))
            (d / "glossary.json").write_text(json.dumps({"terms": [{"term": "Models", "key": "common:nav.models"}]}))
            self.assertEqual(i18n.glossary("de", d), [("Models", "Models", "Modelle")])
            self.assertEqual(i18n.check(d), [])

    def test_check_finds_a_bad_glossary(self):
        with tempfile.TemporaryDirectory() as tmp:
            d = pathlib.Path(tmp)
            catalog(d, "en", "common", {"nav": {"models": "Models"}})
            (d / "languages.json").write_text(json.dumps([{"code": "en"}]))
            terms = [{"term": "Models", "key": "common:nav.models"}, {"term": "Models", "key": "common:nav.gone"}]
            (d / "glossary.json").write_text(json.dumps({"terms": terms}))
            problems = i18n.check(d)
        self.assertEqual(len(problems), 2)
        self.assertIn("common:nav.gone", problems[0])
        self.assertIn("listed twice", problems[1])

    def test_kept_text_is_not_counted(self):
        english = {"chat:pdf": "PDF", "chat:name": "Name", "chat:gone": "Not translated yet"}
        german = {"chat:pdf": "PDF", "chat:name": "Name", "chat:gone": "Not translated yet"}
        listed = {"all": ["chat:pdf"], "de": ["chat:name"]}
        _, same = i18n.status(english, german, i18n.kept("de", listed))
        self.assertEqual(same, ["chat:gone"])
        # Another language keeps only what is under all.
        _, same = i18n.status(english, german, i18n.kept("fr", listed))
        self.assertEqual(same, ["chat:gone", "chat:name"])

    def test_check_finds_a_stale_same_as_english_list(self):
        with tempfile.TemporaryDirectory() as tmp:
            d = pathlib.Path(tmp)
            catalog(d, "en", "common", {"nav": {"models": "Models", "name": "Name", "pdf": "PDF"}})
            catalog(d, "de", "common", {"nav": {"models": "Modelle", "name": "Name", "pdf": "PDF"}})
            (d / "languages.json").write_text(json.dumps([{"code": "en"}, {"code": "de"}]))
            (d / "glossary.json").write_text(json.dumps({"terms": []}))
            listed = {
                "about": "text",
                "all": ["common:nav.pdf", "common:nav.gone"],
                "de": ["common:nav.name", "common:nav.models", "common:nav.pdf"],
                "xx": ["common:nav.name"],
            }
            (d / "same-as-english.json").write_text(json.dumps(listed))
            problems = i18n.check(d)
        self.assertEqual(len(problems), 4, problems)
        self.assertIn("all lists common:nav.gone", problems[0])
        self.assertIn("de's common:nav.models is translated now", problems[1])
        self.assertIn("de lists common:nav.pdf, which is already under all", problems[2])
        self.assertIn("xx is not in languages.json", problems[3])

    def test_the_real_glossary(self):
        self.assertEqual(i18n.check(), [])


if __name__ == "__main__":
    unittest.main()
