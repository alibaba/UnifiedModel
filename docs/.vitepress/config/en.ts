import type { LocaleSpecificConfig, DefaultTheme } from 'vitepress'

const nav: DefaultTheme.NavItem[] = [
  { text: 'Guide', link: '/en/getting-started/quickstart', activeMatch: '/en/(getting-started|concepts|guides|architecture)/' },
  { text: 'Reference', link: '/en/reference/cli', activeMatch: '/en/reference/' },
  { text: 'Specs', link: '/en/spec/plan-schema-v1', activeMatch: '/en/spec/' },
  {
    text: 'Demos',
    items: [
      { text: 'Incident Investigation', link: 'https://github.com/alibaba/UnifiedModel/tree/main/examples/incident-investigation' },
      { text: 'Service Localization', link: 'https://github.com/alibaba/UnifiedModel/tree/main/examples/service-localization' },
      { text: 'Multi-Domain Quickstart', link: 'https://github.com/alibaba/UnifiedModel/tree/main/examples/quickstart-multidomain' },
    ],
  },
]

const sidebar: DefaultTheme.SidebarItem[] = [
  {
    text: 'Getting Started',
    items: [
      { text: 'Installation', link: '/en/getting-started/installation' },
      { text: 'Quick Start', link: '/en/getting-started/quickstart' },
      { text: 'GraphStore Providers', link: '/en/graphstore-providers' },
    ],
  },
  {
    text: 'Concepts',
    collapsed: false,
    items: [
      { text: 'Concepts Index', link: '/en/concepts/' },
      { text: 'Object Graph Semantic Layer', link: '/en/concepts/object-graph-semantic-layer' },
      { text: 'Workspaces And Domains', link: '/en/concepts/workspaces-and-domains' },
      { text: 'Model Elements', link: '/en/concepts/model-elements' },
      { text: 'Entity Sets', link: '/en/concepts/entity-sets' },
      { text: 'Datasets', link: '/en/concepts/datasets' },
      { text: 'Links And Field Mappings', link: '/en/concepts/links-and-field-mappings' },
      { text: 'Storage And GraphStore', link: '/en/concepts/storage-and-graphstore' },
      { text: 'Entities And Relations', link: '/en/concepts/entities-and-relations' },
      { text: 'Query Surfaces', link: '/en/concepts/query-surfaces' },
    ],
  },
  {
    text: 'Guides',
    collapsed: true,
    items: [
      { text: 'Model Authoring', link: '/en/guides/model-authoring' },
      { text: 'Entity And Relation Writes', link: '/en/guides/entity-relation-writes' },
      { text: 'Query Service', link: '/en/guides/query-service' },
      { text: 'Web UI', link: '/en/guides/web-ui' },
      { text: 'SDK And Client', link: '/en/guides/sdk-clients' },
      { text: 'Agent Integration', link: '/en/guides/agent-integration' },
    ],
  },
  {
    text: 'Architecture',
    collapsed: true,
    items: [
      { text: 'Overview', link: '/en/architecture/overview' },
      { text: 'Runtime Flow', link: '/en/architecture/runtime-flow' },
      { text: 'Query And Agent', link: '/en/architecture/query-and-agent' },
      { text: 'Extension Points', link: '/en/architecture/extension-points' },
    ],
  },
  {
    text: 'Reference',
    collapsed: true,
    items: [
      { text: 'CLI', link: '/en/reference/cli' },
      { text: 'MCP', link: '/en/reference/mcp' },
      { text: 'Web UI API Map', link: '/en/ui-api' },
      { text: 'UI Architecture', link: '/en/ui-architecture' },
      { text: 'SDK Specification', link: '/en/umodel-sdk-specification' },
    ],
  },
  {
    text: 'Specifications',
    collapsed: true,
    items: [
      { text: 'Plan Schema v1', link: '/en/spec/plan-schema-v1' },
    ],
  },
]

export const en: LocaleSpecificConfig<DefaultTheme.Config> & { label: string; link?: string } = {
  label: 'English',
  lang: 'en-US',
  link: '/en/',
  themeConfig: {
    nav,
    sidebar: { '/en/': sidebar },
    outline: { level: [2, 3], label: 'On this page' },
    docFooter: { prev: 'Previous', next: 'Next' },
    editLink: {
      pattern: 'https://github.com/alibaba/UnifiedModel/edit/main/docs/:path',
      text: 'Edit this page on GitHub',
    },
    footer: {
      message: 'Released under the Apache-2.0 License.',
      copyright: 'Copyright © 2026 UnifiedModel (UModel) maintainers',
    },
  },
}
