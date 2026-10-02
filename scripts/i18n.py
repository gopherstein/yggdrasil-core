#!/usr/bin/env python3
"""Translation status and glossary for the shared catalog in i18n/.

The web tests (web/src/i18n/catalog.test.ts) check that a language is
correct: valid JSON, English's keys, placeholders, and plural forms. This
script says what a language still lacks, to help someone translating or
reviewing it:

- missing: keys English has and the language does not, which show in
  English until they are translated;
- same as English: text identical to English, which is often not translated
  yet, though some is meant to stay, such as "Chat" or "API".

Usage:
    scripts/i18n.py status [LANGUAGE ...] [--keys]
                                       what each language lacks; --keys lists them
    scripts/i18n.py glossary LANGUAGE  the glossary's terms in a language
    scripts/i18n.py check              every glossary key exists in English
"""

from __future__ import annotations

import argparse
import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
I18N = ROOT / "i18n"
SOURCE = "en"

PLURAL = re.compile(r"_(zero|one|two|few|many|other)$")
PLACEHOLDER = re.compile(r"\{\{[^}]*\}\}")


def flatten(value: object, prefix: str = "", out: dict[str, str] | None = None) -> dict[str, str]:
    out = {} if out is None else out
    if isinstance(value, dict):
        for k, v in value.items():
            flatten(v, f"{prefix}.{k}" if prefix else k, out)
    else:
        out[prefix] = str(value)
    return out


def load(language: str, i18n: pathlib.Path = I18N) -> dict[str, str]:
    """A language's text by "namespace:key"."""
    out: dict[str, str] = {}
    for path in sorted((i18n / "locales" / language).glob("*.json")):
        for key, text in flatten(json.loads(path.read_text(encoding="utf-8"))).items():
            out[f"{path.stem}:{key}"] = text
    return out


def languages(i18n: pathlib.Path = I18N) -> list[dict]:
    return json.loads((i18n / "languages.json").read_text(encoding="utf-8"))


def base(key: str) -> str:
    """A plural key's name without its form: chat:steps.memories_other → chat:steps.memories."""
    return PLURAL.sub("", key)


def has_words(text: str) -> bool:
    """Text with something to translate, not only placeholders, numbers, or symbols."""
    return any(c.isalpha() for c in PLACEHOLDER.sub("", text))


def status(english: dict[str, str], translated: dict[str, str]) -> tuple[list[str], list[str]]:
    """The keys a language is missing, and those whose text is the same as English.

    A plural key counts as present when the language has any of its forms,
    because languages use different forms; the web tests check the forms.
    """
    have = {base(k) for k in translated}
    missing = sorted({base(k) for k in english if base(k) not in have})
    same = sorted(k for k, text in translated.items() if english.get(k) == text and has_words(text))
    return missing, same


def glossary(language: str, i18n: pathlib.Path = I18N) -> list[tuple[str, str, str]]:
    """Each glossary term with its English text and its text in language."""
    terms = json.loads((i18n / "glossary.json").read_text(encoding="utf-8"))["terms"]
    english, translated = load(SOURCE, i18n), load(language, i18n)
    return [(t["term"], english.get(t["key"], ""), translated.get(t["key"], "")) for t in terms]


def check(i18n: pathlib.Path = I18N) -> list[str]:
    """Problems with the glossary: a key English lacks, or a term listed twice."""
    terms = json.loads((i18n / "glossary.json").read_text(encoding="utf-8"))["terms"]
    english = load(SOURCE, i18n)
    problems, seen = [], set()
    for t in terms:
        if t["key"] not in english:
            problems.append(f"glossary: {t['term']!r} names {t['key']}, which English does not have")
        if t["term"] in seen:
            problems.append(f"glossary: {t['term']!r} is listed twice")
        seen.add(t["term"])
    return problems


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)
    p_status = sub.add_parser("status", help="what each language lacks")
    p_status.add_argument("languages", nargs="*")
    p_status.add_argument("--keys", action="store_true", help="list the keys")
    p_glossary = sub.add_parser("glossary", help="the glossary in a language")
    p_glossary.add_argument("language")
    sub.add_parser("check", help="check the glossary")
    args = parser.parse_args(argv)

    listed = {lang["code"]: lang for lang in languages()}
    if args.command == "check":
        problems = check()
        for p in problems:
            print(p, file=sys.stderr)
        return 1 if problems else 0

    if args.command == "glossary":
        if args.language not in listed:
            print(f"{args.language} is not in i18n/languages.json", file=sys.stderr)
            return 1
        rows = glossary(args.language)
        width = max(len(term) for term, _, _ in rows)
        for term, english, text in rows:
            print(f"{term:<{width}}  {english}  →  {text or '(missing)'}")
        return 0

    english = load(SOURCE)
    wanted = args.languages or [code for code in listed if code != SOURCE]
    for code in wanted:
        if code not in listed:
            print(f"{code} is not in i18n/languages.json", file=sys.stderr)
            return 1
    print(f"English has {len({base(k) for k in english})} keys.")
    for code in wanted:
        missing, same = status(english, load(code))
        print(f"{code:<8} {listed[code].get('status', ''):<10} missing {len(missing):>4}   same as English {len(same):>4}")
        if args.keys:
            for k in missing:
                print(f"    missing  {k}")
            for k in same:
                print(f"    same     {k}  {english[k]!r}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
