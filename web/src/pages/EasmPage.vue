<template>
  <div class="easm-page">
    <header class="top">
      <div class="brand"><b>EASM</b><span>外部攻击面态势</span></div>
      <form class="search" @submit.prevent="lookupDomain">
        <input v-model="query" placeholder="输入主域，反查备案主体" />
        <button type="submit">查询</button>
      </form>
      <select v-model="riskFilter" class="risk-filter" @change="loadMap">
        <option value="">全部风险</option>
        <option value="medium">中危及以上</option>
        <option value="high">高危及以上</option>
        <option value="critical">仅高危</option>
      </select>
      <HealthPill :health="health" />
    </header>

    <div v-if="banner" class="banner">{{ banner }}</div>

    <div v-if="lookupResult" class="lookup-card">
      <div class="lookup-main">
        <b>{{ lookupResult.attribution.orgName }}</b>
        <span class="muted">{{ lookupResult.attribution.icpNo || '无备案号' }} · 置信度 {{ lookupResult.confidence }} · 来源 {{ lookupResult.provider || 'ICP' }}</span>
      </div>
      <div class="lookup-actions">
        <button @click="openLookup">打开组织</button>
        <button @click="refreshICP">反查全部备案主域</button>
        <button class="ghost" @click="lookupResult = null">关闭</button>
      </div>
    </div>

    <div class="body">
      <OrgRail :orgs="orgs" :selected-id="selectedOrgId" :bubbles="snapshot.organizations" @select="selectOrg" />
      <main class="canvas">
        <WorldMap
          :organizations="snapshot.organizations"
          :points="snapshot.points"
          :graph="graph"
          :graph-on="graphOn"
          :selected-org-id="selectedOrgId"
          @select-org="selectOrg"
          @select-point="selectPoint"
          @select-node="selectNode"
        />
        <div class="map-tools">
          <button :class="{ active: graphOn }" :disabled="!selectedOrgId || health.neo4j !== 'ok'" :title="health.neo4j !== 'ok' ? '关系图暂不可用' : ''" @click="toggleGraph">
            图谱
          </button>
          <button v-if="selectedNode && selectedNode.entityId && ['domain','ip','service'].includes(selectedNode.type)" class="ghost" @click="openNodeAsset">打开资产</button>
          <button v-if="selectedOrgId" class="ghost" @click="openOrganization">组织详情</button>
          <button v-if="selectedOrgId" class="ghost" @click="clearSelection">全局视图</button>
        </div>
        <Legend class="legend" />
        <div class="stats">
          未定位资产 {{ snapshot.unlocatedAssets }} · 未定位组织 {{ snapshot.unlocatedOrganizations }} · 未关联漏洞 {{ snapshot.unlinkedFindings }}<span v-if="snapshot.truncated"> · 数据已截断</span>
        </div>
      </main>
      <Inspector
        v-if="showInspector"
        :organization="selectedOrg"
        :domains="domains"
        :point="selectedPoint"
        :node="selectedNode"
        :job="job"
        :busy="busy"
        @close="clearSelection"
        @refresh="refreshICP"
        @confirm="confirmDomains"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, qs } from '@/api'
import HealthPill from '@/components/easm/HealthPill.vue'
import Inspector from '@/components/easm/Inspector.vue'
import Legend from '@/components/easm/Legend.vue'
import OrgRail from '@/components/easm/OrgRail.vue'
import WorldMap from '@/components/easm/WorldMap.vue'
import type {
  GraphNode, GraphSnapshot, Health, ICPJob, LookupResult, MapPoint, MapSnapshot,
  Organization, OrganizationDomain,
} from '@/components/easm/types'

const router = useRouter()
const orgs = ref<Organization[]>([])
const selectedOrgId = ref('')
const railBubbles = ref<MapSnapshot['organizations']>([])
const snapshot = ref<MapSnapshot>({
  organizations: [], points: [], unlocatedAssets: 0, unlocatedOrganizations: 0, unlinkedFindings: 0, updatedAt: '',
})
const graph = ref<GraphSnapshot | null>(null)
const graphOn = ref(false)
const selectedPoint = ref<MapPoint | null>(null)
const selectedNode = ref<GraphNode | null>(null)
const domains = ref<OrganizationDomain[]>([])
const job = ref<ICPJob | null>(null)
const health = ref<Health>({ chinaz: 'unavailable', icpQuery: 'unavailable', geoip: 'unavailable', neo4j: 'unavailable' })
const query = ref('')
const riskFilter = ref('')
const lookupResult = ref<LookupResult | null>(null)
const banner = ref('')
const busy = ref(false)
let pollTimer: ReturnType<typeof setInterval> | undefined

const selectedOrg = computed(() => orgs.value.find((o) => o.id === selectedOrgId.value) || lookupResult.value?.organization || null)
const showInspector = computed(() => Boolean(selectedOrgId.value || selectedPoint.value || selectedNode.value))

async function loadHealth() {
  try {
    const res = await api<{ data: Health }>('/easm/health')
    health.value = res.data
  } catch {
    health.value = { chinaz: 'unavailable', icpQuery: 'unavailable', geoip: 'unavailable', neo4j: 'unavailable' }
  }
}

async function loadOrgs() {
  const res = await api<{ data: { items: Organization[] } }>('/organizations')
  orgs.value = res.data.items || []
}

async function loadMap() {
  const res = await api<{ data: MapSnapshot }>(`/easm/map${qs({
    orgId: selectedOrgId.value || undefined, risk: riskFilter.value || undefined,
  })}`)
  snapshot.value = res.data
}

async function loadGlobalBubbles() {
  const res = await api<{ data: MapSnapshot }>('/easm/map')
  railBubbles.value = res.data.organizations || []
}

async function loadDomains() {
  if (!selectedOrgId.value) { domains.value = []; return }
  const res = await api<{ data: { items: OrganizationDomain[] } }>(`/organizations/${selectedOrgId.value}/domains`)
  domains.value = res.data.items || []
}

async function loadGraph() {
  if (!graphOn.value || !selectedOrgId.value) { graph.value = null; return }
  const res = await api<{ data: GraphSnapshot }>(`/easm/graph${qs({ orgId: selectedOrgId.value, depth: 2, limit: 300 })}`)
  graph.value = res.data
}

async function selectOrg(id: string) {
  selectedOrgId.value = id
  selectedPoint.value = null
  selectedNode.value = null
  job.value = null
  await Promise.all([loadMap(), loadDomains(), loadGraph()])
}

function selectPoint(point: MapPoint) {
  selectedPoint.value = point
  selectedNode.value = null
}

function selectNode(node: GraphNode) {
  selectedNode.value = node
  selectedPoint.value = null
}

function openOrganization() {
  if (!selectedOrgId.value) return
  router.push(`/easm/organizations/${selectedOrgId.value}`)
}

function openNodeAsset() {
  if (!selectedNode.value?.entityId) return
  router.push({ path: '/easm/assets', query: { focus: selectedNode.value.entityId, organizationId: selectedOrgId.value } })
}

function clearSelection() {
  selectedOrgId.value = ''
  selectedPoint.value = null
  selectedNode.value = null
  graphOn.value = false
  graph.value = null
  domains.value = []
  void loadMap()
}

async function toggleGraph() {
  if (!selectedOrgId.value) return
  if (health.value.neo4j !== 'ok') {
    banner.value = '关系图暂不可用，请检查 Neo4j 连接'
    return
  }
  graphOn.value = !graphOn.value
  await loadGraph()
}

async function lookupDomain() {
  const domain = query.value.trim()
  if (!domain) return
  banner.value = ''
  busy.value = true
  try {
    const res = await api<{ data: LookupResult }>('/icp/lookup', {
      method: 'POST', body: JSON.stringify({ domain }),
    })
    lookupResult.value = res.data
  } catch (e: any) {
    banner.value = e.message || 'ICP 查询失败'
  } finally {
    busy.value = false
  }
}

async function openLookup() {
  if (!lookupResult.value) return
  await loadOrgs()
  const target = orgs.value.find((o) => o.id === lookupResult.value?.organization.id)
  if (target) await selectOrg(target.id)
}

async function refreshICP() {
  const orgId = selectedOrgId.value || lookupResult.value?.organization.id
  if (!orgId) return
  banner.value = ''
  busy.value = true
  try {
    const res = await api<{ data: ICPJob }>(`/organizations/${orgId}/icp-domains/refresh`, { method: 'POST' })
    job.value = res.data
    await pollJob(res.data.id)
  } catch (e: any) {
    banner.value = e.message || '备案反查失败'
  } finally {
    busy.value = false
  }
}

async function pollJob(id: string) {
  for (let i = 0; i < 30; i++) {
    const res = await api<{ data: ICPJob }>(`/icp/jobs/${id}`)
    job.value = res.data
    if (res.data.status === 'done' || res.data.status === 'failed') {
      await Promise.all([loadDomains(), loadMap()])
      return
    }
    await new Promise((resolve) => setTimeout(resolve, 2000))
  }
}

async function confirmDomains(list: string[], createTask: boolean) {
  if (!selectedOrgId.value || !list.length) return
  busy.value = true
  banner.value = ''
  try {
    await api(`/organizations/${selectedOrgId.value}/domains/confirm`, {
      method: 'POST', body: JSON.stringify({ domains: list }),
    })
    if (createTask) {
      await api('/tasks', {
        method: 'POST',
        body: JSON.stringify({ name: `EASM ${selectedOrg.value?.canonicalName || ''}`, kind: 'domain', target: list.join('\n') }),
      })
    }
    await Promise.all([loadDomains(), loadMap()])
  } catch (e: any) {
    banner.value = e.message || '确认失败'
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  try {
    await Promise.all([loadOrgs(), loadHealth(), loadGlobalBubbles()])
    if (orgs.value.length) await selectOrg(orgs.value[0].id)
    else await loadMap()
  } catch (e: any) {
    banner.value = e.message || '加载 EASM 数据失败'
  }
  pollTimer = setInterval(() => {
    void loadMap()
    void loadGlobalBubbles()
    if (graphOn.value) void loadGraph()
    void loadHealth()
  }, 30000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<style scoped>
.easm-page {
  margin: -24px;
  min-height: calc(100vh - 68px);
  display: flex;
  flex-direction: column;
  background: #070d1a;
  color: #cfe0ff;
  font-size: 13px;
}
.top {
  height: 54px;
  flex: 0 0 54px;
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 0 18px;
  border-bottom: 1px solid #12294a;
  background: #081226;
}
.brand { display: flex; align-items: baseline; gap: 8px; }
.brand b { font-size: 16px; letter-spacing: .14em; color: #eaf2ff; }
.brand span { font-size: 11px; color: #5f7aa8; }
.search { flex: 1; max-width: 460px; display: flex; }
.search input { flex: 1; background: #0b1830; border: 1px solid #1d3f66; border-right: 0; border-radius: 6px 0 0 6px; color: #cfe0ff; padding: 8px 11px; outline: none; }
.search button { background: #2f6fe4; border: 1px solid #2f6fe4; color: #fff; border-radius: 0 6px 6px 0; padding: 0 16px; cursor: pointer; }
.risk-filter { background: #0b1830; border: 1px solid #1d3f66; color: #9db9e8; border-radius: 6px; padding: 7px 9px; }
.banner { background: #3a1722; color: #ffb3c1; border-bottom: 1px solid #7a2a3a; padding: 7px 18px; font-size: 12px; }
.lookup-card { display: flex; justify-content: space-between; gap: 12px; align-items: center; padding: 9px 18px; background: #0b1830; border-bottom: 1px solid #12294a; }
.lookup-main { display: flex; gap: 10px; align-items: baseline; }
.lookup-main b { color: #eaf2ff; }
.lookup-actions { display: flex; gap: 8px; }
.lookup-actions button { background: #10233d; border: 1px solid #2b5fae; color: #9fc4ff; border-radius: 5px; padding: 5px 9px; cursor: pointer; }
.lookup-actions .ghost { background: transparent; border-color: #1d3f66; color: #7f9fd6; }
.body { flex: 1; display: flex; min-height: 0; }
.canvas { flex: 1; position: relative; min-width: 0; }
.map-tools { position: absolute; top: 12px; right: 12px; display: flex; gap: 6px; }
.map-tools button { background: #0b1830; border: 1px solid #1d3f66; color: #9db9e8; padding: 6px 10px; border-radius: 5px; cursor: pointer; }
.map-tools button.active { border-color: #2f6fe4; color: #eaf2ff; background: #122a47; }
.map-tools .ghost { color: #5f7aa8; }
.legend { position: absolute; left: 14px; bottom: 12px; right: 14px; }
.stats { position: absolute; right: 14px; bottom: 36px; color: #5f7aa8; font-size: 11px; }
.muted { color: #5f7aa8; }
</style>
