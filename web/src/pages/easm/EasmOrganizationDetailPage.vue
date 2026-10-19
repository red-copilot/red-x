<template>
  <div class="easm-page-shell">
    <div class="page-title">
      <div>
        <a-space>
          <a-button type="text" @click="$router.push('/easm/organizations')">← 组织列表</a-button>
          <h1>{{ organization?.canonicalName || '组织详情' }}</h1>
        </a-space>
        <div class="muted">{{ organization?.region || '未设置地区' }} · {{ organization?.aliases?.join('、') || '无别名' }}</div>
      </div>
      <a-space>
        <a-button @click="exportReport('xlsx')">导出 XLSX</a-button>
        <a-button @click="exportReport('csv')">导出 CSV</a-button>
        <a-button type="primary" @click="openScan">发起扫描</a-button>
      </a-space>
    </div>

    <a-alert v-if="banner" type="error" show-icon closable @close="banner = ''">{{ banner }}</a-alert>

    <a-tabs v-model:activeKey="tab" @change="loadTab">
      <a-tab-pane key="overview" tab="概览">
        <a-row :gutter="[12, 12]" class="stats-grid">
          <a-col :span="4"><a-card><a-statistic title="已确认主域" :value="summary?.confirmedDomains || 0" /></a-card></a-col>
          <a-col :span="4"><a-card><a-statistic title="候选主域" :value="summary?.candidateDomains || 0" /></a-card></a-col>
          <a-col :span="4"><a-card><a-statistic title="域名资产" :value="summary?.domainAssets || 0" /></a-card></a-col>
          <a-col :span="4"><a-card><a-statistic title="IP 资产" :value="summary?.ipAssets || 0" /></a-card></a-col>
          <a-col :span="4"><a-card><a-statistic title="服务" :value="summary?.services || 0" /></a-card></a-col>
          <a-col :span="4"><a-card><a-statistic title="当前漏洞" :value="summary?.currentFindings || 0" :value-style="{ color: '#cf1322' }" /></a-card></a-col>
        </a-row>
        <a-row :gutter="[12, 12]" class="stats-grid">
          <a-col :span="6"><a-card><a-statistic title="严重" :value="summary?.criticalFindings || 0" /></a-card></a-col>
          <a-col :span="6"><a-card><a-statistic title="高危" :value="summary?.highFindings || 0" /></a-card></a-col>
          <a-col :span="6"><a-card><a-statistic title="关联任务" :value="summary?.linkedTasks || 0" /></a-card></a-col>
          <a-col :span="6"><a-card><a-statistic title="未定位资产" :value="summary?.unlocatedAssets || 0" /></a-card></a-col>
        </a-row>
        <a-card title="最近任务" :bordered="false" class="tab-card">
          <a-table :data-source="recentTasks" :columns="taskColumns" row-key="id" size="small" :pagination="false">
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'name'"><a @click="$router.push(`/tasks/${record.id}`)">{{ record.name }}</a></template>
              <template v-else-if="column.dataIndex === 'status'"><a-tag>{{ record.status }}</a-tag></template>
              <template v-else-if="column.dataIndex === 'createdAt'">{{ formatTime(record.createdAt) }}</template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <a-tab-pane key="domains" tab="域名治理">
        <a-card :bordered="false" class="tab-card">
          <div class="toolbar">
            <a-space>
              <a-button type="primary" :disabled="!selectedCandidates.length" @click="confirmSelected">确认所选（{{ selectedCandidates.length }}）</a-button>
              <a-button @click="addOpen = true">添加域名</a-button>
              <a-button @click="loadDomains">刷新</a-button>
            </a-space>
            <span class="muted">手工添加只生成候选，确认后才计入资产闭包。</span>
          </div>
          <a-table :data-source="domains" :columns="domainColumns" row-key="id" size="middle" :pagination="{ pageSize: 20 }">
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'selection'">
                <a-checkbox v-if="record.status === 'candidate'" :checked="selectedCandidates.includes(record.rootDomain)" @change="toggleCandidate(record.rootDomain)" />
              </template>
              <template v-else-if="column.dataIndex === 'rootDomain'"><span class="mono">{{ record.rootDomain }}</span></template>
              <template v-else-if="column.dataIndex === 'confidence'"><a-tag :color="confidenceColor(record.confidence)">{{ record.confidence }}</a-tag></template>
              <template v-else-if="column.dataIndex === 'status'"><a-tag :color="statusColor(record.status)">{{ statusText(record.status, record.reassignedTo) }}</a-tag></template>
              <template v-else-if="column.dataIndex === 'source'">{{ record.source }} · {{ formatTime(record.lastSeen) }}</template>
              <template v-else-if="column.dataIndex === 'actions'">
                <a-space>
                  <a-button size="small" @click="openEvidence(record)">证据</a-button>
                  <a-button v-if="record.status !== 'rejected'" size="small" danger @click="rejectDomain(record)">拒绝/移出</a-button>
                  <a-button v-if="record.status === 'confirmed'" size="small" @click="openReassign(record)">重归属</a-button>
                </a-space>
              </template>
              <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <a-tab-pane key="assets" tab="资产">
        <a-card :bordered="false" class="tab-card">
          <div class="toolbar">
            <a-space wrap>
              <a-input-search v-model:value="assetQuery" placeholder="搜索域名、IP、服务" @search="loadAssets(1)" />
              <a-select v-model:value="assetKind" style="width: 140px" @change="loadAssets(1)">
                <a-select-option value="">全部类型</a-select-option>
                <a-select-option value="domain">域名</a-select-option>
                <a-select-option value="ip">IP</a-select-option>
                <a-select-option value="service">服务</a-select-option>
              </a-select>
              <a-select v-model:value="assetRisk" style="width: 140px" @change="loadAssets(1)">
                <a-select-option value="">全部风险</a-select-option>
                <a-select-option value="high">高危及以上</a-select-option>
                <a-select-option value="critical">仅严重</a-select-option>
              </a-select>
            </a-space>
          </div>
          <a-table :data-source="assetPage.items" :columns="assetColumns" row-key="id" size="middle" :pagination="pager(assetPage, loadAssets)">
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'key'"><span class="mono">{{ record.key }}</span></template>
              <template v-else-if="column.dataIndex === 'risk'"><a-tag :color="riskColor(record.risk)">{{ record.risk || '无' }}</a-tag></template>
              <template v-else-if="column.dataIndex === 'lastSeen'">{{ formatTime(record.lastSeen) }}</template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <a-tab-pane key="findings" tab="风险">
        <a-card :bordered="false" class="tab-card">
          <div class="toolbar">
            <a-space wrap>
              <a-input-search v-model:value="findingQuery" placeholder="搜索标题、目标、插件" @search="loadFindings(1)" />
              <a-select v-model:value="findingSeverity" style="width: 140px" @change="loadFindings(1)">
                <a-select-option value="">全部级别</a-select-option>
                <a-select-option v-for="s in ['critical', 'high', 'medium', 'low', 'info']" :key="s" :value="s">{{ s }}</a-select-option>
              </a-select>
              <a-select v-model:value="findingKind" style="width: 150px" @change="loadFindings(1)">
                <a-select-option value="">全部类型</a-select-option>
                <a-select-option v-for="k in ['vuln', 'nuclei', 'wih', 'file_leak', 'protocol']" :key="k" :value="k">{{ k }}</a-select-option>
              </a-select>
            </a-space>
          </div>
          <a-table :data-source="findingPage.items" :columns="findingColumns" row-key="finding.id" size="middle" :pagination="pager(findingPage, loadFindings)" @row-click="openHistory">
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'title'"><a>{{ record.finding.title }}</a></template>
              <template v-else-if="column.dataIndex === 'target'"><span class="mono">{{ record.finding.target }}</span></template>
              <template v-else-if="column.dataIndex === 'severity'"><a-tag :color="riskColor(record.finding.severity)">{{ record.finding.severity }}</a-tag></template>
              <template v-else-if="column.dataIndex === 'plugin'">{{ record.finding.pluginName || '-' }}</template>
              <template v-else-if="column.dataIndex === 'occurrences'">{{ record.occurrences }}</template>
              <template v-else-if="column.dataIndex === 'createdAt'">{{ formatTime(record.finding.createdAt) }}</template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <a-tab-pane key="tasks" tab="任务">
        <a-card :bordered="false" class="tab-card">
          <div class="toolbar"><a-button @click="loadTasks">刷新</a-button></div>
          <a-table :data-source="taskPage.items" :columns="taskColumns" row-key="id" size="middle" :pagination="pager(taskPage, loadTasks)">
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'name'"><a @click="$router.push(`/tasks/${record.id}`)">{{ record.name }}</a></template>
              <template v-else-if="column.dataIndex === 'status'"><a-tag>{{ record.status }}</a-tag></template>
              <template v-else-if="column.dataIndex === 'progress'"><a-progress :percent="record.progress || 0" size="small" /></template>
              <template v-else-if="column.dataIndex === 'createdAt'">{{ formatTime(record.createdAt) }}</template>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>
    </a-tabs>

    <a-modal v-model:open="addOpen" title="添加备案主域" :confirm-loading="busy" @ok="addDomains">
      <p class="muted">每行一个主域或 URL；添加后仍需人工确认。</p>
      <a-textarea v-model:value="domainInput" :rows="8" placeholder="example.com" />
    </a-modal>

    <a-modal v-model:open="reassignOpen" title="重归属域名" :confirm-loading="busy" @ok="reassignDomain">
      <a-form layout="vertical">
        <a-form-item label="目标组织" required>
          <a-select v-model:value="reassignTarget" show-search :filter-option="filterOrg" placeholder="选择目标组织">
            <a-select-option v-for="org in organizations" :key="org.id" :value="org.id" :disabled="org.id === orgId">{{ org.canonicalName }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="原因"><a-input v-model:value="reassignReason" /></a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="evidenceOpen" title="ICP 归属证据" :footer="null" width="760px">
      <a-descriptions v-if="evidence?.domain" size="small" bordered :column="2">
        <a-descriptions-item label="主域">{{ evidence.domain.rootDomain }}</a-descriptions-item>
        <a-descriptions-item label="来源">{{ evidence.domain.source }}</a-descriptions-item>
        <a-descriptions-item label="备案号">{{ evidence.domain.icpNo || '-' }}</a-descriptions-item>
        <a-descriptions-item label="置信度">{{ evidence.domain.confidence }}</a-descriptions-item>
      </a-descriptions>
      <pre class="evidence-raw">{{ evidence?.evidence?.raw || '无原始证据' }}</pre>
    </a-modal>

    <a-modal v-model:open="scanOpen" title="发起 EASM 扫描" :confirm-loading="busy" width="680px" @ok="createScans">
      <a-form layout="vertical">
        <a-form-item label="已确认主域" required>
          <a-checkbox-group v-model:value="scanDomains" class="scan-domains">
            <a-checkbox v-for="row in confirmedDomains" :key="row.id" :value="row.rootDomain">{{ row.rootDomain }}</a-checkbox>
          </a-checkbox-group>
        </a-form-item>
        <a-form-item label="Policy ID"><a-input v-model:value="scanForm.policyId" placeholder="可选" /></a-form-item>
        <a-form-item label="Scope ID"><a-input v-model:value="scanForm.scopeId" placeholder="可选" /></a-form-item>
        <a-form-item label="扫描选项">
          <a-space wrap>
            <a-checkbox v-model:checked="scanForm.options.domainBrute">域名爆破</a-checkbox>
            <a-checkbox v-model:checked="scanForm.options.portScan">端口扫描</a-checkbox>
            <a-checkbox v-model:checked="scanForm.options.serviceDetection">服务识别</a-checkbox>
            <a-checkbox v-model:checked="scanForm.options.siteIdentify">指纹识别</a-checkbox>
            <a-checkbox v-model:checked="scanForm.options.nucleiScan">Nuclei</a-checkbox>
          </a-space>
        </a-form-item>
      </a-form>
    </a-modal>

    <a-drawer v-model:open="historyOpen" title="漏洞历史" width="720">
      <a-timeline>
        <a-timeline-item v-for="item in history" :key="item.id">
          <b>{{ item.severity }}</b> · {{ item.title }} · {{ formatTime(item.createdAt) }}
          <div class="muted mono">{{ item.target }}</div>
        </a-timeline-item>
      </a-timeline>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useRoute } from 'vue-router'
import { api, formatTime, qs } from '@/api'
import type {
  AssetPage, EasmAsset, EasmTask, FindingItem, FindingPage, Organization,
  OrganizationDomain, OrganizationSummary,
} from '@/components/easm/types'

const route = useRoute()
const orgId = String(route.params.id)
const tab = ref('overview')
const banner = ref('')
const busy = ref(false)
const organization = ref<Organization | null>(null)
const organizations = ref<Organization[]>([])
const domains = ref<OrganizationDomain[]>([])
const summary = ref<OrganizationSummary | null>(null)
const assetPage = ref<AssetPage>({ items: [], total: 0, page: 1, pageSize: 20, truncated: false, unlocatedAssets: 0 })
const findingPage = ref<FindingPage>({ items: [], total: 0, page: 1, pageSize: 20, truncated: false, unlinkedFindings: 0 })
const taskPage = ref<{ items: EasmTask[]; total: number; page: number; pageSize: number }>({ items: [], total: 0, page: 1, pageSize: 20 })
const selectedCandidates = ref<string[]>([])
const assetQuery = ref('')
const assetKind = ref('')
const assetRisk = ref('')
const findingQuery = ref('')
const findingSeverity = ref('')
const findingKind = ref('')
const addOpen = ref(false)
const reassignOpen = ref(false)
const evidenceOpen = ref(false)
const scanOpen = ref(false)
const historyOpen = ref(false)
const domainInput = ref('')
const reassignTarget = ref('')
const reassignReason = ref('')
const evidence = ref<any>(null)
const history = ref<any[]>([])
const reassignRow = ref<OrganizationDomain | null>(null)
const scanDomains = ref<string[]>([])
const scanForm = reactive({
  policyId: '',
  scopeId: '',
  options: { domainBrute: true, portScan: true, serviceDetection: false, siteIdentify: true, nucleiScan: false },
})

const confirmedDomains = computed(() => domains.value.filter((d) => d.status === 'confirmed'))
const recentTasks = computed(() => taskPage.value.items.slice(0, 5))
const domainColumns = [
  { title: '', dataIndex: 'selection', width: 42 },
  { title: '主域', dataIndex: 'rootDomain', width: 220 },
  { title: '状态', dataIndex: 'status', width: 120 },
  { title: '置信度', dataIndex: 'confidence', width: 100 },
  { title: '来源 / 最近发现', dataIndex: 'source', width: 240 },
  { title: 'ICP 备案号', dataIndex: 'icpNo', width: 180 },
  { title: '操作', dataIndex: 'actions', width: 250 },
]
const assetColumns = [
  { title: '类型', dataIndex: 'kind', width: 90 },
  { title: '资产', dataIndex: 'key', width: 300 },
  { title: '标题', dataIndex: 'title', width: 220 },
  { title: '风险', dataIndex: 'risk', width: 90 },
  { title: '主域', dataIndex: 'rootDomains', width: 220 },
  { title: '最近发现', dataIndex: 'lastSeen', width: 180 },
]
const findingColumns = [
  { title: '标题', dataIndex: 'title', width: 280 },
  { title: '目标', dataIndex: 'target', width: 280 },
  { title: '级别', dataIndex: 'severity', width: 90 },
  { title: '插件', dataIndex: 'plugin', width: 160 },
  { title: '出现次数', dataIndex: 'occurrences', width: 90 },
  { title: '最近发现', dataIndex: 'createdAt', width: 180 },
]
const taskColumns = [
  { title: '任务', dataIndex: 'name', width: 280 },
  { title: '主域', dataIndex: 'rootDomain', width: 200 },
  { title: '状态', dataIndex: 'status', width: 110 },
  { title: '进度', dataIndex: 'progress', width: 160 },
  { title: '创建时间', dataIndex: 'createdAt', width: 180 },
]

async function loadBase() {
  const [orgRes, sumRes, orgsRes] = await Promise.all([
    api<{ data: { organization: Organization; domains: OrganizationDomain[] } }>(`/organizations/${orgId}`),
    api<{ data: OrganizationSummary }>(`/easm/organizations/${orgId}/summary`),
    api<{ data: { items: Organization[] } }>('/organizations?limit=200'),
  ])
  organization.value = orgRes.data.organization
  domains.value = orgRes.data.domains || []
  summary.value = sumRes.data
  organizations.value = orgsRes.data.items || []
  selectedCandidates.value = domains.value.filter((d) => d.status === 'candidate').map((d) => d.rootDomain)
}

async function loadDomains() {
  const res = await api<{ data: { items: OrganizationDomain[] } }>(`/organizations/${orgId}/domains`)
  domains.value = res.data.items || []
  selectedCandidates.value = domains.value.filter((d) => d.status === 'candidate').map((d) => d.rootDomain)
}

async function loadAssets(page = assetPage.value.page) {
  const res = await api<{ data: AssetPage }>(`/easm/organizations/${orgId}/assets${qs({
    page, pageSize: assetPage.value.pageSize, kind: assetKind.value, risk: assetRisk.value, q: assetQuery.value,
  })}`)
  assetPage.value = res.data
}

async function loadFindings(page = findingPage.value.page) {
  const res = await api<{ data: FindingPage }>(`/easm/organizations/${orgId}/findings${qs({
    page, pageSize: findingPage.value.pageSize, state: 'current', severity: findingSeverity.value,
    kind: findingKind.value, q: findingQuery.value,
  })}`)
  findingPage.value = res.data
}

async function loadTasks(page = taskPage.value.page) {
  const res = await api<{ data: { items: EasmTask[]; total: number; page: number; pageSize: number } }>(`/easm/organizations/${orgId}/tasks${qs({
    page, pageSize: taskPage.value.pageSize,
  })}`)
  taskPage.value = res.data
}

async function loadTab() {
  if (tab.value === 'assets') await loadAssets(1)
  if (tab.value === 'findings') await loadFindings(1)
  if (tab.value === 'tasks') await loadTasks(1)
}

function pager(source: any, loader: (page: number) => void) {
  return {
    current: source.page || 1,
    pageSize: source.pageSize || 20,
    total: source.total || 0,
    showSizeChanger: false,
    onChange: (page: number) => loader(page),
  }
}

function toggleCandidate(value: string) {
  selectedCandidates.value = selectedCandidates.value.includes(value)
    ? selectedCandidates.value.filter((x) => x !== value)
    : [...selectedCandidates.value, value]
}

async function confirmSelected() {
  if (!selectedCandidates.value.length) return
  busy.value = true
  try {
    await api(`/organizations/${orgId}/domains/confirm`, { method: 'POST', body: JSON.stringify({ domains: selectedCandidates.value }) })
    message.success('域名已确认')
    await Promise.all([loadDomains(), loadBase()])
  } catch (e: any) {
    banner.value = e.message || '确认失败'
  } finally {
    busy.value = false
  }
}

async function addDomains() {
  const list = domainInput.value.split(/\s+/).map((x) => x.trim()).filter(Boolean)
  if (!list.length) return
  busy.value = true
  try {
    const res = await api<{ data: { imported: number; rejected: string[] } }>(`/organizations/${orgId}/domains/import`, {
      method: 'POST', body: JSON.stringify({ domains: list }),
    })
    message.success(`已加入 ${res.data.imported} 个候选域名`)
    addOpen.value = false
    domainInput.value = ''
    await loadDomains()
  } catch (e: any) {
    banner.value = e.message || '添加失败'
  } finally {
    busy.value = false
  }
}

async function rejectDomain(row: OrganizationDomain) {
  const reason = window.prompt('拒绝/移出原因', '人工移出') || ''
  busy.value = true
  try {
    await api(`/organizations/${orgId}/domains/${row.id}/reject`, { method: 'POST', body: JSON.stringify({ reason }) })
    await Promise.all([loadDomains(), loadBase()])
  } catch (e: any) {
    banner.value = e.message || '操作失败'
  } finally {
    busy.value = false
  }
}

function openReassign(row: OrganizationDomain) {
  reassignRow.value = row
  reassignTarget.value = ''
  reassignReason.value = '归属修正'
  reassignOpen.value = true
}

async function reassignDomain() {
  if (!reassignRow.value || !reassignTarget.value) { message.warning('请选择目标组织'); return }
  busy.value = true
  try {
    await api(`/organizations/${orgId}/domains/${reassignRow.value.id}/reassign`, {
      method: 'POST', body: JSON.stringify({ targetOrganizationId: reassignTarget.value, reason: reassignReason.value }),
    })
    message.success('已重归属')
    reassignOpen.value = false
    await Promise.all([loadDomains(), loadBase()])
  } catch (e: any) {
    banner.value = e.message || '重归属失败'
  } finally {
    busy.value = false
  }
}

async function openEvidence(row: OrganizationDomain) {
  try {
    const res = await api<{ data: any }>(`/organizations/${orgId}/domains/${row.id}/evidence`)
    evidence.value = res.data
  } catch (e: any) {
    evidence.value = { domain: row, evidence: { raw: e.message || '无证据' } }
  }
  evidenceOpen.value = true
}

function openScan() {
  scanDomains.value = confirmedDomains.value.slice(0, 10).map((d) => d.rootDomain)
  scanOpen.value = true
}

async function createScans() {
  if (!scanDomains.value.length) { message.warning('请选择已确认主域'); return }
  busy.value = true
  try {
    const res = await api<{ data: { items: EasmTask[]; errors?: any[] } }>(`/easm/organizations/${orgId}/scans`, {
      method: 'POST',
      body: JSON.stringify({ rootDomains: scanDomains.value, policyId: scanForm.policyId, scopeId: scanForm.scopeId, options: scanForm.options }),
    })
    message.success(`已创建 ${res.data.items.length} 个任务`)
    if (res.data.errors?.length) banner.value = `${res.data.errors.length} 个主域创建任务失败`
    scanOpen.value = false
    await Promise.all([loadTasks(1), loadBase()])
  } catch (e: any) {
    banner.value = e.message || '扫描创建失败'
  } finally {
    busy.value = false
  }
}

async function openHistory(record: FindingItem) {
  const owner = record.organizationIds?.[0] || orgId
  try {
    const res = await api<{ data: { items: any[] } }>(`/easm/organizations/${owner}/findings/${record.finding.id}/history`)
    history.value = res.data.items || []
  } catch (e: any) {
    history.value = []
    banner.value = e.message || '加载历史失败'
  }
  historyOpen.value = true
}

async function exportReport(format: 'xlsx' | 'csv') {
  try {
    const res = await api<{ data: { downloadUrl: string } }>(`/easm/organizations/${orgId}/exports`, {
      method: 'POST', body: JSON.stringify({ format }),
    })
    window.open(res.data.downloadUrl, '_blank')
  } catch (e: any) {
    banner.value = e.message || '导出失败'
  }
}

function statusText(status: string, reassignedTo?: string) {
  if (reassignedTo) return '已重归属'
  return { confirmed: '已确认', candidate: '待确认', rejected: '已拒绝' }[status] || status
}
function statusColor(status: string) { return { confirmed: 'green', candidate: 'orange', rejected: 'default' }[status] || 'default' }
function confidenceColor(confidence: string) { return { high: 'green', medium: 'orange', low: 'red' }[confidence] || 'default' }
function riskColor(risk?: string) { return { critical: 'red', high: 'volcano', medium: 'orange', low: 'blue', info: 'default' }[risk || ''] || 'default' }
function filterOrg(input: string, option: any) { return String(option?.children || '').toLowerCase().includes(input.toLowerCase()) }

onMounted(async () => {
  try {
    await Promise.all([loadBase(), loadTasks(1)])
    await loadTab()
  } catch (e: any) {
    banner.value = e.message || '加载组织失败'
  }
})
</script>

<style scoped>
.easm-page-shell { display: flex; flex-direction: column; gap: 14px; }
.stats-grid { margin-bottom: 12px; }
.tab-card { border-radius: 14px; }
.toolbar { display: flex; justify-content: space-between; gap: 12px; margin-bottom: 14px; flex-wrap: wrap; }
.evidence-raw { max-height: 420px; overflow: auto; background: #0a1526; color: #cfe0ff; padding: 12px; border-radius: 8px; white-space: pre-wrap; }
.scan-domains { display: flex; flex-direction: column; gap: 6px; }
</style>
