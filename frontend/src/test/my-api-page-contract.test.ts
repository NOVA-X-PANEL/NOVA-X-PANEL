import { readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

/*
 * Two bugs shipped in the "My API" page, and both were invisible to every other
 * check the project runs. Neither is a logic error — the page's logic was fine.
 * One was a missing import, the other a wrong HTTP method paired with a `catch`
 * that hid the resulting 404.
 *
 * That combination is worth a test of its own, because:
 *
 *  - TypeScript cannot see either. `import '@/x.css'` has no type, and
 *    `HttpUtil.post` and `HttpUtil.get` are both valid calls.
 *  - The build cannot see either. Both compile.
 *  - A render test cannot see the first one. The components render; they are
 *    simply unstyled, which jsdom does not model (it has no layout and no CSS).
 *
 * So the assertions here read the source. That is a weaker kind of test than
 * exercising behaviour, and it is used deliberately: these two defects are
 * *contract* defects against the rest of the codebase — "this file needs that
 * stylesheet", "this route is a GET" — and a contract is exactly what a source
 * assertion can pin.
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

const STYLE_LAYER = '@/pg-ui/styles/pasarguard.css';

describe('My API page — the style layer', () => {
  it('imports the Tailwind layer, which is what defines the colour tokens', () => {
    const source = read('pages/my-api/MyApiPage.tsx');

    // Every component this page renders (Button, Card, Dialog, Switch, Badge,
    // Separator, Skeleton) is a Tailwind/shadcn component, and this file is the
    // only place the semantic tokens (--card, --muted-foreground, --border) are
    // defined. Without it the page falls back to unstyled HTML, and the Radix
    // Dialog — a portal with no positioning of its own — renders off-screen, so
    // the page appears to ignore every click.
    expect(source).toContain(STYLE_LAYER);
  });

  it('is the only page mounting a pg-ui feature, so the general rule has one member', () => {
    // If a second page starts mounting a pg-ui feature directly, the rule below
    // begins covering it too. Asserting the membership keeps that visible: a new
    // member is a signal, not a silent widening of what this test protects.
    const mounters = walk(join(srcRoot, 'pages'), ['.tsx']).filter((file) =>
      readFileSync(file, 'utf8').includes('@/pg-ui/features/'),
    );

    expect(mounters.map((f) => f.slice(srcRoot.length + 1))).toEqual([
      'pages/my-api/MyApiPage.tsx',
    ]);
  });
});

describe('My API page — the style layer is pulled in by every pg-ui mount', () => {
  it('any page mounting a pg-ui feature brings the style layer with it', () => {
    // The general form of the bug. A page that mounts a pg-ui *shell*
    // (`@/pg-ui/pages/_dashboard.*`) gets the stylesheet transitively, because
    // those shells import it. A page that mounts a pg-ui *feature* directly gets
    // nothing, and that is precisely what shipped broken.
    const roots = [join(srcRoot, 'pages'), join(srcRoot, 'layouts')];
    const offenders: string[] = [];

    for (const root of roots) {
      for (const file of walk(root, ['.tsx', '.ts'])) {
        const source = stripComments(readFileSync(file, 'utf8'));
        const mountsFeature = source.includes('@/pg-ui/features/');
        if (!mountsFeature) continue;

        const bringsStyles = source.includes(STYLE_LAYER) || source.includes('@/pg-ui/pages/');
        if (!bringsStyles) {
          offenders.push(file.slice(srcRoot.length + 1));
        }
      }
    }

    expect(offenders, 'these mount a pg-ui feature without the Tailwind layer').toEqual([]);
  });
});

describe('My API page — the token list contract', () => {
  const componentPath = 'pg-ui/features/admins/components/my-api-tokens.tsx';

  it('reads the list with GET, matching the route the server registers', () => {
    const source = stripComments(read(componentPath));

    // The route is `g.GET("/apiTokens")`. A POST to a GET-only route is a 404,
    // and the old `catch { return [] }` turned that 404 into a permanently empty
    // list — "No tokens yet." forever, with no error to explain it.
    expect(source).toContain("HttpUtil.get('/panel/api/admins/apiTokens'");
    expect(source).not.toMatch(/HttpUtil\.post\(\s*['"`]\/panel\/api\/admins\/apiTokens['"`]/);
  });

  it('registers the GET route on the server, so the client contract holds', () => {
    // Pinned from both sides on purpose: this test is what makes a future change
    // to either side fail loudly instead of silently breaking the page again.
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
});
