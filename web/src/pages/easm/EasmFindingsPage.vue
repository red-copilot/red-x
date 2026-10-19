<template>
  <div class="easm-page-shell">
    <div class="page-title">
      <div><h1>EASM 风险</h1><span class="muted">按 target + dedupeKey 聚合的当前漏洞态</span></div>
    </div>
    <a-card :bordered="false" class="tab-card">
      <div class="toolbar">
        <a-input-search v-model:value="query" placeholder="搜索标题、目标、插件" style="max-width: 320px" @search="load(1)" />
        <a-select v-model:value="orgId" show-search allow-clear placeholder="全部组织" style="width: 240px" :filter-option="filterOrg" @change="load(1)">
          <a-select-option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.canonicalName }}</a-select-option>
        </a-select>
        <a-select v-model:value="severity" placeholder="级别" allow-clear style="width: 140px" @change="load(1)">
          <a-select-option v-for="s in ['critical', 'high', 'medium', 'low', 'info']" :key="s" :value="s">{{ s }}</a-select-option>
        </a-select>
        <a-select v-model:value="kind" placeholder="类型" allow-clear style="width: 150px" @change="load(1)">
          <a-select-option v-for="k in ['vuln', 'nuclei', 'wih', 'file_leak', 'protocol']" :key="k" :value="k">{{ k }}</a-select-option>
        </a-select>
        <a-button @click="load(page)">刷新</a-button>
      </div>
      <a-table :data-source="data.items" :columns="columns" :row-key="(r: FindingItem) => r.finding.id" size="middle" :pagination="pagination" @change="onChange" @row-click="openHistory">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'title'"><a>{{ record.finding.title }}</a></template>
          <template v-else-if="column.dataIndex === 'target'"><span class="mono">{{ record.finding.target }}</span></template>
          <template v-else-if="column.dataIndex === 'severity'"><a-tag :color="riskColor(record.finding.severity)">{{ record.finding.severity }}</a-tag></template>
          <template v-else-if="column.dataIndex === 'plugin'">{{ record.finding.pluginName || '-' }}</template>
          <template v-else-if="column.dataIndex === 'occurrences'">{{ record.occurrences }}</template>
          <template v-else-if="column.dataIndex === 'organizations'">{{ record.organizationIds.length }}</template>
          <template v-else-if="column.dataIndex === 'createdAt'">{{ formatTime(record.finding.createdAt) }}</template>
        </template>
      </a-table>
    </a-card>
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
import { onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { api, formatTime, qs } from '@/api'
import type { FindingItem, FindingPage, Organization } from '@/components/easm/types'

const organizations = ref<Organization[]>([])
const orgId = ref<string | undefined>()
const severity = ref<string | undefined>()
const kind = ref<string | undefined>()
const query = ref('')
const page = ref(1)
const pageSize = ref(20)
const data = ref<FindingPage>({ items: [], total: 0, page: 1, pageSize: 20, truncated: false, unlinkedFindings: 0 })
const history = ref<any[]>([])
const historyOpen = ref(false)
const columns = [
  { title: '标题', dataIndex: 'title', width: 280 },
  { title: '目标', dataIndex: 'target', width: 300 },
  { title: '级别', dataIndex: 'severity', width: 90 },
  { title: '插件', dataIndex: 'plugin', width: 160 },
  { title: '出现次数', dataIndex: 'occurrences', width: 90 },
  { title: '组织数', dataIndex: 'organizations', width: 80 },
  { title: '最近发现', dataIndex: 'createdAt', width: 180 },
]
const pagination = ref({ current: 1, pageSize: 20, total: 0, showSizeChanger: false })

async function loadOrganizations() {
  const res = await api<{ data: { items: Organization[] } }>('/organizations?limit=200')
  organizations.value = res.data.items || []
}

async function load(next = page.value) {
  page.value = next
  const res = await api<{ data: FindingPage }>(`/easm/findings${qs({
    organizationId: orgId.value, severity: severity.value, kind: kind.value, q: query.value,
    state: 'current', page: page.value, pageSize: pageSize.value,
  })}`)
  data.value = res.data
  pagination.value = { current: res.data.page, pageSize: res.data.pageSize, total: res.data.total, showSizeChanger: false }
}

async function openHistory(record: FindingItem) {
  const owner = record.organizationIds?.[0]
  if (!owner) { message.warning('漏洞未关联组织'); return }
  const res = await api<{ data: { items: any[] } }>(`/easm/organizations/${owner}/findings/${record.finding.id}/history`)
  history.value = res.data.items || []
  historyOpen.value = true
}

function onChange(p: any) { load(p.current) }
function riskColor(value?: string) { return { critical: 'red', high: 'volcano', medium: 'orange', low: 'blue', info: 'default' }[value || ''] || 'default' }
function filterOrg(input: string, option: any) { return String(option?.children || '').toLowerCase().includes(input.toLowerCase()) }

onMounted(async () => {
  await loadOrganizations()
  await load(1)
})
</script>

<style scoped>
.easm-page-shell { display: flex; flex-direction: column; gap: 14px; }
.tab-card { border-radius: 14px; }
.toolbar { display: flex; gap: 10px; flex-wrap: wrap; margin-bottom: 16px; }
</style>
