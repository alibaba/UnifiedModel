import { Activity, Box, Braces, Database, FileJson, GitBranch } from 'lucide-react'

export function iconForKind(kind: string) {
  if (kind.includes('metric')) return <Activity size={13} />
  if (kind.includes('log') || kind.includes('trace') || kind.includes('event') || kind.includes('profile')) return <Database size={13} />
  if (kind.includes('link')) return <GitBranch size={13} />
  if (kind === 'explorer') return <FileJson size={13} />
  if (kind === 'entity_set') return <Box size={13} />
  return <Braces size={13} />
}
