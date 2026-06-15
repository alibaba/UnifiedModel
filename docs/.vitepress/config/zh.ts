import type { LocaleSpecificConfig, DefaultTheme } from 'vitepress'
import schemaSidebar from './schema-sidebar.json'

const nav: DefaultTheme.NavItem[] = [
  { text: '指南', link: '/zh/getting-started/quickstart', activeMatch: '/zh/(getting-started|concepts|guides|architecture)/' },
  { text: '参考', link: '/zh/reference/cli', activeMatch: '/zh/reference/' },
  { text: '规范', link: '/zh/spec/plan-schema-v1', activeMatch: '/zh/spec/' },
  {
    text: '示例',
    items: [
      { text: '故障排查 Demo', link: 'https://github.com/alibaba/UnifiedModel/tree/main/examples/incident-investigation' },
      { text: '服务定位 Demo', link: 'https://github.com/alibaba/UnifiedModel/tree/main/examples/service-localization' },
      { text: '多域 Quickstart', link: 'https://github.com/alibaba/UnifiedModel/tree/main/examples/quickstart-multidomain' },
    ],
  },
]

const sidebar: DefaultTheme.SidebarItem[] = [
  {
    text: '入门',
    items: [
      { text: '安装', link: '/zh/getting-started/installation' },
      { text: '快速开始', link: '/zh/getting-started/quickstart' },
      { text: 'GraphStore Providers', link: '/zh/graphstore-providers' },
    ],
  },
  {
    text: '概念',
    collapsed: false,
    items: [
      { text: '概念索引', link: '/zh/concepts/' },
      { text: '对象图语义层', link: '/zh/concepts/object-graph-semantic-layer' },
      { text: 'Workspace 与 Domain', link: '/zh/concepts/workspaces-and-domains' },
      { text: '模型元素', link: '/zh/concepts/model-elements' },
      { text: 'EntitySet 实体集', link: '/zh/concepts/entity-sets' },
      { text: '数据集', link: '/zh/concepts/datasets' },
      { text: '链接与字段映射', link: '/zh/concepts/links-and-field-mappings' },
      { text: '存储与 GraphStore', link: '/zh/concepts/storage-and-graphstore' },
      { text: '实体与关系', link: '/zh/concepts/entities-and-relations' },
      { text: '查询入口', link: '/zh/concepts/query-surfaces' },
    ],
  },
  {
    text: '指南',
    collapsed: true,
    items: [
      { text: '模型编写', link: '/zh/guides/model-authoring' },
      { text: '实体与关系写入', link: '/zh/guides/entity-relation-writes' },
      { text: 'Query Service', link: '/zh/guides/query-service' },
      { text: 'Web UI', link: '/zh/guides/web-ui' },
      { text: 'SDK 与客户端', link: '/zh/guides/sdk-clients' },
      { text: 'Agent 集成', link: '/zh/guides/agent-integration' },
    ],
  },
  {
    text: '架构',
    collapsed: true,
    items: [
      { text: '总览', link: '/zh/architecture/overview' },
      { text: '运行时流程', link: '/zh/architecture/runtime-flow' },
      { text: 'Query 与 Agent', link: '/zh/architecture/query-and-agent' },
      { text: '扩展点', link: '/zh/architecture/extension-points' },
    ],
  },
  {
    text: '参考',
    collapsed: true,
    items: [
      { text: 'CLI', link: '/zh/reference/cli' },
      { text: 'MCP', link: '/zh/reference/mcp' },
      { text: 'Schema', link: '/zh/reference/schema/' },
      { text: 'Web UI API 映射', link: '/zh/ui-api' },
      { text: 'UI 架构', link: '/zh/ui-architecture' },
      { text: 'SDK 规范', link: '/zh/umodel-sdk-specification' },
    ],
  },
  {
    text: '规范',
    collapsed: true,
    items: [
      { text: 'Plan Schema v1', link: '/zh/spec/plan-schema-v1' },
    ],
  },
]

const schemaNav: DefaultTheme.SidebarItem[] = [
  { text: 'Schema 参考', link: '/zh/reference/schema/' },
  ...(schemaSidebar.zh as DefaultTheme.SidebarItem[]),
]

export const zh: LocaleSpecificConfig<DefaultTheme.Config> & { label: string; link?: string } = {
  label: '简体中文',
  lang: 'zh-CN',
  link: '/zh/',
  themeConfig: {
    nav,
    sidebar: { '/zh/reference/schema/': schemaNav, '/zh/': sidebar },
    outline: { level: [2, 3], label: '本页目录' },
    docFooter: { prev: '上一页', next: '下一页' },
    editLink: {
      pattern: 'https://github.com/alibaba/UnifiedModel/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页',
    },
    footer: {
      message: '基于 Apache-2.0 许可发布。',
      copyright: 'Copyright © 2026 UnifiedModel (UModel) maintainers',
    },
    lastUpdatedText: '最后更新',
    returnToTopLabel: '返回顶部',
    sidebarMenuLabel: '菜单',
    darkModeSwitchLabel: '主题',
    langMenuLabel: '切换语言',
  },
}
