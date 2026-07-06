import { describe, it, expect } from 'vitest';

// Same harness as dialectic.test.ts: this project's tsconfig only pulls in
// "vite/client" types and vitest mocks `*.css?raw` to empty, so read the
// stylesheet through node builtins via string-variable specifiers.
declare const process: { cwd(): string };

const fs = (await import('node:fs' as string)) as {
  readFileSync(path: string, encoding: 'utf8'): string;
};
const path = (await import('node:path' as string)) as {
  resolve(...segments: string[]): string;
};
const raw = fs.readFileSync(
  path.resolve(process.cwd(), 'src/styles/dialectic.css'),
  'utf8',
);
// Strip comments so prose in them (which mentions `backdrop-filter`) never
// counts as a declaration.
const css = raw.replace(/\/\*[\s\S]*?\*\//g, '');

// Grab a rule body by its selector (up to the next `}`).
function ruleBody(selector: string): string {
  const at = css.indexOf(selector);
  expect(at, `selector ${selector} missing`).toBeGreaterThan(-1);
  const open = css.indexOf('{', at);
  const close = css.indexOf('}', open);
  return css.slice(open + 1, close);
}

describe('Liquid Glass v2 — nav chrome consumes the shared glass tokens', () => {
  it('the nav bar is a thin-glass capsule drawn from --glass-* tokens', () => {
    const bar = ruleBody('.v-dialectic .nav-inner {');
    expect(bar).toContain('var(--glass-bg-thin)');
    expect(bar).toMatch(/backdrop-filter:\s*blur\(var\(--glass-blur-thin\)\)/);
    expect(bar).toContain('var(--glass-saturate)');
    expect(bar).toContain('var(--glass-border)');
    expect(bar).toContain('var(--shadow-glass)');
    expect(bar).toContain('var(--radius-pill)');
  });

  it('the nav shell itself is transparent (the capsule floats, no frost on the shell)', () => {
    const shell = ruleBody('.v-dialectic .nav {');
    expect(shell).toMatch(/background:\s*transparent/);
    expect(shell).not.toContain('backdrop-filter');
  });

  it('nav links are pills whose active/hover state is pill-fill + specular edge', () => {
    const link = ruleBody('.v-dialectic .nav-link {');
    expect(link).toContain('var(--radius-pill)');
    const hover = ruleBody('.v-dialectic .nav-link:hover {');
    expect(hover).toContain('var(--glass-pill-fill)');
    expect(hover).toContain('var(--glass-edge-top)');
  });

  it('primary buttons are smoked-ink capsules (label flips with the theme via tokens)', () => {
    const primary = ruleBody('.v-dialectic .btn-primary {');
    expect(primary).toContain('var(--glass-smoke-strong)');
    expect(primary).toContain('var(--glass-smoke-ink)');
    expect(ruleBody('.v-dialectic .btn {')).toContain('var(--radius-pill)');
  });

  it('secondary (ghost) buttons are thin-glass capsules', () => {
    const ghost = ruleBody('.v-dialectic .btn-ghost {');
    expect(ghost).toContain('var(--glass-bg-thin)');
    expect(ghost).toContain('var(--glass-border)');
    expect(ghost).toContain('var(--glass-edge-top)');
  });
});

describe('Liquid Glass contract — no glass is ever laid over content', () => {
  // Every backdrop-filter in the landing stylesheet must belong to a floating
  // *chrome* surface. Content (transcript, code, diagrams, tables, cards,
  // prose) must never be frosted. This scan fails the moment any new selector
  // that is not on the chrome allowlist gains a backdrop-filter.
  const CHROME = /\.(nav|nav-inner|nav-links|nav-link|btn-ghost|hero-stamp)\b/;

  function selectorOwning(idx: number): string {
    const braceOpen = css.lastIndexOf('{', idx);
    // Selector starts after the nearest preceding `}` or enclosing `{`
    // (handles rules nested inside an @media block).
    const start =
      Math.max(
        css.lastIndexOf('}', braceOpen - 1),
        css.lastIndexOf('{', braceOpen - 1),
      ) + 1;
    return css.slice(start, braceOpen).trim();
  }

  it('every backdrop-filter is on a chrome surface, none on content', () => {
    const hits: number[] = [];
    let i = css.indexOf('backdrop-filter');
    while (i !== -1) {
      hits.push(i);
      i = css.indexOf('backdrop-filter', i + 1);
    }
    expect(hits.length, 'expected the chrome to use backdrop-filter').toBeGreaterThan(0);
    for (const at of hits) {
      const sel = selectorOwning(at);
      expect(CHROME.test(sel), `backdrop-filter on non-chrome selector: "${sel}"`).toBe(true);
    }
  });

  it('the content surfaces carry no backdrop-filter', () => {
    for (const sel of [
      '.v-dialectic .tx-body {',
      '.v-dialectic .code {',
      '.v-dialectic .hook-diagram {',
      '.v-dialectic .compare {',
      '.v-dialectic .stage {',
      '.v-dialectic .pillar {',
    ]) {
      expect(ruleBody(sel)).not.toContain('backdrop-filter');
    }
  });
});
