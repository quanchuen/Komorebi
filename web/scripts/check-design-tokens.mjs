#!/usr/bin/env node
/*
 * Design-token guard.
 *
 * Fails when a Svelte component reaches for an *arbitrary* Tailwind value
 * (e.g. `text-[10px]`, `bg-[#1e293b]`, `w-[200px]`), a raw palette class
 * (e.g. `bg-slate-800`, `text-white`, `accent-red-400`), or hard-codes a hex
 * color in a `style=` attribute. These are the presentational escape hatches that
 * fork the design system; components should use the semantic tokens defined in
 * src/app.css (bg-surface, text-muted, text-2xs, …) instead.
 *
 * Deliberately narrow: it does NOT ban hex everywhere, because MapLibre paint
 * expressions in <script> (Map.svelte, lib/utils/conditionColors.ts) use hex
 * legitimately. Map.svelte is excluded outright; only `class`/`style` usage is
 * inspected elsewhere.
 *
 * Strict by default (exit 1 on any finding). Set CHECK_TOKENS_WARN=1 to report
 * without failing (used by the pre-commit hook for fast, non-blocking feedback).
 */
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('..', import.meta.url));
const scanDirs = [join(root, 'src', 'lib'), join(root, 'src', 'routes')];

// Files whose hex/inline usage is legitimate (MapLibre paint, canvas styling).
const EXCLUDE = new Set(['Map.svelte']);

// Tailwind arbitrary value: a utility ending in `-[ ... ]`. The required hyphen
// before the bracket avoids matching JS array indexing / type annotations.
const ARBITRARY_VALUE = /\b[a-z][a-z0-9-]*-\[[^\]\s]+\]/g;
// Hex color inside a style attribute (style="…#fff…" or style={`…#fff…`}).
const STYLE_HEX = /style=["'{`][^"'}`]*#[0-9a-fA-F]{3,8}/;
// Raw Tailwind palette class (bg-slate-800, text-white, accent-red-400, …).
// The semantic tokens in src/app.css are the only sanctioned color source.
const RAW_PALETTE =
  /\b(?:[a-z-]+:)*(?:bg|text|border|ring|fill|stroke|from|via|to|accent|caret|divide|outline|decoration|shadow)-(?:(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d{2,3}|white|black)(?:\/\d{1,3})?\b/g;

function walk(dir) {
  const out = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) out.push(...walk(full));
    else if (entry.endsWith('.svelte')) out.push(full);
  }
  return out;
}

const findings = [];
for (const file of scanDirs.flatMap(walk)) {
  if (EXCLUDE.has(file.split('/').pop())) continue;
  const rel = relative(root, file);
  const lines = readFileSync(file, 'utf8').split('\n');
  lines.forEach((line, i) => {
    for (const m of line.matchAll(ARBITRARY_VALUE)) {
      findings.push({ rel, line: i + 1, snippet: m[0], kind: 'arbitrary-value' });
    }
    if (STYLE_HEX.test(line)) {
      findings.push({ rel, line: i + 1, snippet: line.trim(), kind: 'style-hex' });
    }
    for (const m of line.matchAll(RAW_PALETTE)) {
      findings.push({ rel, line: i + 1, snippet: m[0], kind: 'raw-palette' });
    }
  });
}

if (findings.length === 0) {
  console.log('✓ design tokens: no arbitrary values or inline hex in components');
  process.exit(0);
}

const warnOnly = process.env.CHECK_TOKENS_WARN === '1';
const label = warnOnly ? 'WARN' : 'ERROR';
console.error(
  `\n${label}: ${findings.length} design-token violation(s) — use tokens from src/app.css:\n`
);
for (const f of findings) {
  console.error(`  ${f.rel}:${f.line}  [${f.kind}]  ${f.snippet}`);
}
console.error('');
process.exit(warnOnly ? 0 : 1);
