import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'
import { shared } from './config/shared'
import { en } from './config/en'
import { zh } from './config/zh'

// UModel documentation site.
// - Content is the existing docs/en + docs/zh trees (single source of truth).
// - English at /en/, Chinese at /zh/ (named locales mapped onto the dirs);
//   the root URL (/) is the landing page (docs/index.md).
// See ./config/shared.ts for base, rewrites, dead-link allowlist, and the
// Vue-interpolation guard; ./config/{en,zh}.ts for nav + sidebar.
const config = withMermaid(
  defineConfig({
    ...shared,
    locales: {
      en,
      zh,
    },
  }),
)

// vitepress-plugin-mermaid force-adds a `debug` entry to Vite's optimizeDeps,
// but mermaid 11 no longer ships it, so dev pre-bundling fails to resolve it and
// blanks the page. Drop the stale entry; the other forced deps (cytoscape, dayjs,
// @braintree/sanitize-url) are made resolvable by docs/.npmrc hoisting.
const include = config.vite?.optimizeDeps?.include
if (include) {
  config.vite!.optimizeDeps!.include = include.filter((dep) => dep !== 'debug')
}

export default config

