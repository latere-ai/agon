import { describe, it, expect } from 'vitest';

// This project's tsconfig only pulls in "vite/client" types (no @types/node).
// Read files through node builtins via string-variable specifiers so TS
// doesn't try to resolve node module types (mirrors dialectic.test.ts).
declare const process: { cwd(): string };

const fs = (await import('node:fs' as string)) as {
  readFileSync(path: string, encoding: 'utf8'): string;
  existsSync(path: string): boolean;
};
const path = (await import('node:path' as string)) as {
  resolve(...segments: string[]): string;
};

const root = process.cwd();
const index = fs.readFileSync(path.resolve(root, 'index.html'), 'utf8');
const fontsCss = fs.readFileSync(path.resolve(root, 'src/styles/fonts.css'), 'utf8');

describe('font loading', () => {
  it('preloads the critical Latin fonts so first paint avoids a FOUT swap', () => {
    for (const href of [
      '/fonts/inter-400.woff2',
      '/fonts/inter-600.woff2',
      '/fonts/instrument-serif-regular.woff2',
      '/fonts/instrument-serif-italic.woff2',
    ]) {
      expect(index).toContain(`<link rel="preload" href="${href}" as="font" type="font/woff2" crossorigin />`);
    }
  });

  it('serves Latin faces as WOFF2 with font-display: block', () => {
    expect(fontsCss).toContain("url('/fonts/inter-400.woff2') format('woff2')");
    expect(fontsCss).toContain("url('/fonts/instrument-serif-regular.woff2') format('woff2')");
    expect(fontsCss).toContain('font-display: block;');
    expect(fontsCss).not.toContain("format('truetype')");
  });

  it('keeps the CJK face as a full-coverage WOFF2 (no glyph-dropping subset)', () => {
    expect(fontsCss).toContain("url('/fonts/lxgw-wenkai-tc-400.woff2') format('woff2')");
    expect(fontsCss).not.toContain('lxgw-wenkai-tc-400-subset');
    expect(fs.existsSync(path.resolve(root, 'public/fonts/lxgw-wenkai-tc-400.woff2'))).toBe(true);
  });

  it('keeps heavy TTF runtime fonts out of public assets', () => {
    for (const filename of [
      'inter-400.ttf',
      'inter-500.ttf',
      'inter-600.ttf',
      'inter-700.ttf',
      'instrument-serif-regular.ttf',
      'instrument-serif-italic.ttf',
      'lxgw-wenkai-tc-400.ttf',
    ]) {
      expect(fs.existsSync(path.resolve(root, 'public/fonts', filename)), filename).toBe(false);
    }
  });
});
