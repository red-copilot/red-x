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
  updatedAt: string
}

export interface GraphNode {
  id: string
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
