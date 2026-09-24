import { readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

/*
 * The My API page shipped broken twice, and neither defect was reachable by any
 * other check the project runs:
 *
 *   1. The page rendered as unstyled HTML, because the Tailwind layer it needs
 *      never reached its bundle.
 *   2. The token list was fetched with `POST` against a `GET`-only route, and the
 *      surrounding `catch` turned the resulting 404 into a permanently empty
 *      list.
 *
 * Neither is a logic error. TypeScript cannot see either — a CSS import has no
 * type and both HTTP methods compile. The build cannot see either. And a render
 * test cannot see the first: jsdom models no layout and no CSS, so an unstyled
 * page and a styled one look identical to it.
 *
 * So these assertions read the source and the built output's contract. That is a
 * weaker kind of test than exercising behaviour, used deliberately: both defects
 * are *contract* defects against the rest of the codebase — "this page needs that
 * stylesheet", "this route is a GET" — and a contract is what a source assertion
 * can pin.
 */

const srcRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const repoRoot = resolve(srcRoot, '..', '..');

function read(relativeToSrc: string): string {
  return readFileSync(join(srcRoot, relativeToSrc), 'utf8');
}

/** Every file under a directory tree, by extension. */
function walk(dir: string, extensions: string[]): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...walk(full, extensions));
    } else if (extensions.some((ext) => entry.endsWith(ext))) {
      out.push(full);
    }
  }
  return out;
}

/** Strips comments so prose about a call cannot be mistaken for the call. */
function stripComments(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

// Both layers a pg-ui screen needs. They are loaded at the entry point for the
// same reason, so they are asserted together.
const STYLE_LAYER = '@/pg-ui/styles/pasarguard.css';
const PHONE_LAYER = '@/styles/pg-admin-mobile.css';

describe('the Tailwind layer is loaded once, at the entry point', () => {
  it('main.tsx imports both layers', () => {
    // This is the guarantee. It is the entry point, so they land in the one CSS
    // chunk that every route loads and no page can be left out.
    const entry = read('main.tsx');
    expect(entry).toContain(STYLE_LAYER);
    expect(entry).toContain(PHONE_LAYER);
  });

  it('the entry imports it eagerly, not inside a lazy route', () => {
    // A dynamic `import()` would be hoisted like the page-level import was, and
    // that is the failure this guards against.
    const entry = stripComments(read('main.tsx'));
    expect(entry).toMatch(/^import\s+['"]@\/pg-ui\/styles\/pasarguard\.css['"]/m);
    expect(entry).toMatch(/^import\s+['"]@\/styles\/pg-admin-mobile\.css['"]/m);
  });

  it('the layer omits Preflight, which is what makes a global load safe', () => {
    // Tailwind's Preflight is a global reset. pasarguard.css deliberately leaves it
    // out so the antd-based screens are untouched — that is the property that lets
    // this file be loaded app-wide. If someone adds Preflight, loading it globally
    // silently restyles every other page in the panel.
    const layer = read('pg-ui/styles/pasarguard.css');
    const imports = layer.match(/@import\s+['"][^'"]+['"]/g) ?? [];
    expect(imports.some((line) => line.includes('preflight'))).toBe(false);
  });

  it('the My API page does NOT import either layer, because that does not work', () => {
    // The page-level imports produced `/* empty css */` in the built chunk: the
    // bundler hoisted the shared stylesheets and attached them to only some
    // routes. Re-adding an import here would look like a fix and change nothing,
    // so their absence is asserted rather than left to memory.
    const page = stripComments(read('pages/my-api/MyApiPage.tsx'));
    expect(page).not.toContain('pasarguard.css');
    expect(page).not.toContain('pg-admin-mobile.css');
  });

  it('every page that renders a pg-ui feature is covered by the entry load', () => {
    // The general rule, stated as a fact about this codebase: the pages which
    // mount pg-ui are the ones that needed the layer, and the entry load covers
    // all of them without any of them importing it.
    const mounters = walk(join(srcRoot, 'pages'), ['.tsx'])
      .filter((file) => readFileSync(file, 'utf8').includes('@/pg-ui/features/'))
      .map((f) => f.slice(srcRoot.length + 1));

    expect(mounters).toEqual(['pages/my-api/MyApiPage.tsx']);
  });
});

describe('the token list contract', () => {
  const componentPath = 'pg-ui/features/admins/components/my-api-tokens.tsx';

  it('reads the list with GET, matching the route the server registers', () => {
    const source = stripComments(read(componentPath));

    // The route is `g.GET("/apiTokens")`. A POST to a GET-only route is a 404, and
    // the old `catch { return [] }` turned that 404 into a permanently empty list
    // — "No tokens yet." forever, with no error to explain it.
    expect(source).toContain("HttpUtil.get('/panel/api/admins/apiTokens'");
    expect(source).not.toMatch(/HttpUtil\.post\(\s*['"`]\/panel\/api\/admins\/apiTokens['"`]/);
  });

  it('registers the GET route on the server, so the client contract holds', () => {
    // Pinned from both sides on purpose: this is what makes a change to either
    // side fail loudly instead of silently breaking the page again.
    const controller = readFileSync(join(repoRoot, 'internal/web/controller/admins.go'), 'utf8');

    expect(controller).toMatch(/g\.GET\("\/apiTokens"/);
    expect(controller).toMatch(/g\.POST\("\/apiTokens\/create"/);
    expect(controller).toMatch(/g\.POST\("\/apiTokens\/delete\/:id"/);
    expect(controller).toMatch(/g\.POST\("\/apiTokens\/setEnabled\/:id"/);
  });

  it('does not swallow a failed load into an empty list', () => {
    const source = stripComments(read(componentPath));

    // The old shape was `try { … } catch { return [] }`, which is what made the
    // 404 invisible. A failure must reach React Query so the list can say why.
    expect(source).not.toMatch(/catch\s*(\([^)]*\))?\s*\{\s*return\s*\[\s*\]/);
    expect(source).toContain('isError');
  });

  it('creates the token with the payload the server binds', () => {
    const source = stripComments(read(componentPath));
    const controller = readFileSync(join(repoRoot, 'internal/web/controller/admins.go'), 'utf8');

    // `ownApiTokenPayload` reads `name` and `expiresAt`, and `expiresAt: 0` means
    // "never". A renamed field would be a silent no-op: the server would accept the
    // request and reject the token with an empty-name error.
    expect(source).toContain("'/panel/api/admins/apiTokens/create'");
    expect(source).toContain('name: tokenName');
    expect(source).toContain('expiresAt: 0');
    expect(controller).toMatch(/Name\s+string\s+`json:"name"`/);
    expect(controller).toMatch(/ExpiresAt\s+int64\s+`json:"expiresAt"`/);
  });
});
