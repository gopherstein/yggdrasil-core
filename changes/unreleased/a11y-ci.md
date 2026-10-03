### Changed

- CI checks every page for accessibility problems. It runs axe-core in a real browser over each page with demo data, in light and dark themes at desktop and phone widths, and fails on contrast, labeling, or structure problems (WCAG 2.2 AA). Run it locally with `node scripts/screenshots/a11y.mjs`; see docs/development.md.
