import type { UserConfig } from 'vitepress'

// Configuration shared across both locales. Locale-specific nav/sidebar live in
// ./en.ts and ./zh.ts. The site reuses the existing docs/en + docs/zh trees as
// content (single source of truth), so the only structural shaping done here is
// README→index routing, the out-of-docs dead-link allowlist, and the Vue
// interpolation guard for literal `{{ }}` in the source docs.
export const shared: UserConfig = {
  title: 'UModel',
  description: 'The vendor-neutral semantic runtime for enterprise AI, data governance, and operational intelligence.',
  base: '/UnifiedModel/',
  cleanUrls: false, // GitHub Pages serves .html reliably; clean URLs are not guaranteed.
  lastUpdated: true,
  metaChunk: true,
  appearance: false, // Light mode only — no dark-mode toggle.

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/UnifiedModel/openumodel-mark.svg' }],
    ['meta', { name: 'theme-color', content: '#3c6df0' }],
  ],

  sitemap: { hostname: 'https://alibaba.github.io/UnifiedModel/' },

  // docs/en + docs/zh use README.md as the folder entry; map the two locale
  // roots to their index URL. docs/README.md (the GitHub language router) is
  // intentionally left untouched.
  rewrites: {
    'en/README.md': 'en/index.md',
    'zh/README.md': 'zh/index.md',
  },

  // The reused docs use literal `{{ }}` inside inline code (e.g.
  // `${{src.service_id}}`). Vue would treat those as interpolation and break the
  // build. Entity-escape braces in inline code only, so they render literally —
  // without changing Vue's global delimiters (which breaks the theme's own
  // `{{ }}`) and without editing the source docs. GitHub rendering is unaffected.
  markdown: {
    config: (md) => {
      md.renderer.rules.code_inline = (tokens, idx, _options, _env, self) => {
        const token = tokens[idx]
        const content = md.utils
          .escapeHtml(token.content)
          .replace(/{/g, '&#123;')
          .replace(/}/g, '&#125;')
        return `<code${self.renderAttrs(token)}>${content}</code>`
      }
    },
  },

  // The canonical docs also render on github.com and link out of the docs tree
  // into sibling repo dirs and source files. Those are valid on GitHub but
  // outside VitePress's srcDir. Allowlist exactly those shapes so genuine
  // in-docs typos still fail the build.
  ignoreDeadLinks: [
    /(?:\.\.\/)+(?:web|sdk|generated|examples|pkg|tools|schemas|deployments|skills)(?:\/|$)/,
    /(?:\.\.\/)+(?:README|README_CN|CONTRIBUTING|CODE_OF_CONDUCT|SECURITY|SUPPORT|CHANGELOG)(?:\.[\w.-]+)?$/,
    // Generated HTML doc bundles (docs/html, docs/html_en, docs/html_cn) linked
    // from docs/README.md with a single "./" prefix.
    /(?:^|\/)html(?:_en|_cn)?\/index$/,
  ],

  themeConfig: {
    logo: '/openumodel-mark.svg',
    search: { provider: 'local' },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/alibaba/UnifiedModel' },
    ],
  },
}
