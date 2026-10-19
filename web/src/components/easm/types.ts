export type Risk = 'critical' | 'high' | 'medium' | 'low' | 'info' | ''

export interface Organization {
  id: string
  canonicalName: string
  aliases?: string[]
  region?: string
  status: string
}

export interface OrganizationDomain {
  id: string
  organizationId: string
  rootDomain: string
  siteName?: string
  icpNo?: string
  auditDate?: string
  source: string
  confidence: string
  status: string
  decisionReason?: string
  decidedBy?: string
  decidedAt?: string
  reassignedTo?: string
  firstSeen?: string
  lastSeen?: string
}

export interface OrgBubble {
  orgId: string
  name: string
  assetCount: number
  risk: Risk
  lat: number
  lon: number
}

export interface MapPoint {
  assetId: string
  key: string
  kind: string
  lat: number
  lon: number
  risk: Risk
  cdn: boolean
  cdnName?: string
  orgIds: string[]
}

export interface MapSnapshot {
  organizations: OrgBubble[]
  points: MapPoint[]
  unlocatedAssets: number
  unlocatedOrganizations: number
  unlinkedFindings: number
  truncated?: boolean
  updatedAt: string
}

export interface GraphNode {
  id: string
  entityId?: string
  type: string
  label: string
  risk?: Risk
  cdn?: boolean
}

export interface GraphEdge {
  id: string
  type: string
  source: string
  target: string
  confidence?: string
}

export interface GraphSnapshot {
  nodes: GraphNode[]
  edges: GraphEdge[]
  truncated: boolean
  aggregated: boolean
}

export interface LookupResult {
  organization: Organization
  attribution: {
    orgName: string
    orgType: string
    icpNo: string
    auditDate: string
  }
  confidence: string
}

export interface ICPJob {
  id: string
  status: string
  resultCount: number
  error?: string
}

export interface Health {
  chinaz: string
  icpQuery: string
  geoip: string
  neo4j: string
}


export interface OrganizationSummary {
  organizationId: string
  confirmedDomains: number
  candidateDomains: number
  domainAssets: number
  ipAssets: number
  services: number
  currentFindings: number
  criticalFindings: number
  highFindings: number
  mediumFindings: number
  lowFindings: number
  infoFindings: number
  historyFindings: number
  unlocatedAssets: number
  linkedTasks: number
  lastScanAt?: string
}

export interface EasmAsset {
  id: string
  kind: string
  key: string
  title?: string
  risk?: Risk
  firstSeen: string
  lastSeen: string
  organizationIds: string[]
  rootDomains: string[]
  data?: Record<string, unknown>
}

export interface AssetPage {
  items: EasmAsset[]
  total: number
  page: number
  pageSize: number
  truncated: boolean
  unlocatedAssets: number
}

export interface FindingItem {
  finding: {
    id: string
    taskId: string
    kind: string
    dedupeKey: string
    title: string
    severity: Risk
    target: string
    pluginName?: string
    data?: Record<string, unknown>
    createdAt: string
  }
  occurrences: number
  organizationIds: string[]
  rootDomains: string[]
}

export interface FindingPage {
  items: FindingItem[]
  total: number
  page: number
  pageSize: number
  truncated: boolean
  unlinkedFindings: number
}

export interface EasmTask {
  id: string
  name: string
  kind: string
  target: string
  rootDomain?: string
  organizationId?: string
  origin?: string
  status: string
  progress: number
  createdAt: string
}

export interface ImportError {
  row: number
  message: string
}

export interface OrganizationImportResult {
  created: number
  updated: number
  domainsQueued: number
  errors?: ImportError[]
}
