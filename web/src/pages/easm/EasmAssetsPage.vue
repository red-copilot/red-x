<template>
  <div class="easm-page-shell">
    <div class="page-title">
      <div><h1>EASM 资产</h1><span class="muted">按组织归属闭包聚合的域名、IP 与服务</span></div>
    </div>
    <a-card :bordered="false" class="tab-card">
      <div class="toolbar">
        <a-input-search v-model:value="query" placeholder="搜索资产" style="max-width: 320px" @search="load(1)" />
        <a-select v-model:value="orgId" show-search allow-clear placeholder="全部组织" style="width: 240px" :filter-option="filterOrg" @change="load(1)">
          <a-select-option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.canonicalName }}</a-select-option>
        </a-select>
        <a-select v-model:value="kind" placeholder="类型" allow-clear style="width: 140px" @change="load(1)">
          <a-select-option value="domain">域名</a-select-option>
          <a-select-option value="ip">IP</a-select-option>
          <a-select-option value="service">服务</a-select-option>
        </a-select>
        <a-select v-model:value="risk" placeholder="风险" allow-clear style="width: 140px" @change="load(1)">
          <a-select-option value="high">高危及以上</a-select-option>
          <a-select-option value="critical">仅严重</a-select-option>
        </a-select>
        <a-button @click="load(page)">刷新</a-button>
      </div>
      <a-alert v-if="data.truncated" type="warning" show-icon>闭包数据超过上限，当前结果已截断。</a-alert>
      <a-table :data-source="data.items" :columns="columns" row-key="id" size="middle" :pagination="pagination" @change="onChange" @row-click="openAsset">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'key'"><span class="mono">{{ record.key }}</span></template>
          <template v-else-if="column.dataIndex === 'risk'"><a-tag :color="riskColor(record.risk)">{{ record.risk || '无' }}</a-tag></template>
          <template v-else-if="column.dataIndex === 'rootDomains'">{{ (record.rootDomains || []).join('、') || '-' }}</template>
          <template v-else-if="column.dataIndex === 'organizationIds'">{{ record.organizationIds.length }}</template>
          <template v-else-if="column.dataIndex === 'lastSeen'">{{ formatTime(record.lastSeen) }}</template>
          <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
        </template>
      </a-table>
    </a-card>
    <a-drawer v-model:open="detailOpen" title="资产详情" width="640">
      <a-descriptions v-if="detail" bordered size="small" :column="1">
        <a-descriptions-item label="类型">{{ detail.kind }}</a-descriptions-item>
        <a-descriptions-item label="资产">{{ detail.key }}</a-descriptions-item>
        <a-descriptions-item label="标题">{{ detail.title || '-' }}</a-descriptions-item>
        <a-descriptions-item label="风险">{{ detail.risk || '无' }}</a-descriptions-item>
        <a-descriptions-item label="组织">{{ detail.organizationIds.join('、') }}</a-descriptions-item>
        <a-descriptions-item label="主域">{{ detail.rootDomains.join('、') || '-' }}</a-descriptions-item>
      </a-descriptions>
      <pre class="asset-data">{{ JSON.stringify(detail?.data || {}, null, 2) }}</pre>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, formatTime, qs } from '@/api'
import type { AssetPage, EasmAsset, Organization } from '@/components/easm/types'

const route = useRoute()
const organizations = ref<Organization[]>([])
const detail = ref<EasmAsset | null>(null)
const detailOpen = ref(false)
const orgId = ref<string | undefined>()
const kind = ref<string | undefined>()
const risk = ref<string | undefined>()
const query = ref('')
const page = ref(1)
const pageSize = ref(20)
const data = ref<AssetPage>({ items: [], total: 0, page: 1, pageSize: 20, truncated: false, unlocatedAssets: 0 })
const columns = [
  { title: '类型', dataIndex: 'kind', width: 90 },
  { title: '资产', dataIndex: 'key', width: 320 },
  { title: '标题', dataIndex: 'title', width: 220 },
  { title: '风险', dataIndex: 'risk', width: 90 },
  { title: '主域', dataIndex: 'rootDomains', width: 240 },
  { title: '组织数', dataIndex: 'organizationIds', width: 80 },
  { title: '最近发现', dataIndex: 'lastSeen', width: 180 },
]
const pagination = ref({ current: 1, pageSize: 20, total: 0, showSizeChanger: false })

async function loadOrganizations() {
  const res = await api<{ data: { items: Organization[] } }>('/organizations?limit=200')
  organizations.value = res.data.items || []
}

async function load(next = page.value) {
  page.value = next
  const res = await api<{ data: AssetPage }>(`/easm/assets${qs({
    organizationId: orgId.value, kind: kind.value, risk: risk.value, q: query.value, page: page.value, pageSize: pageSize.value,
  })}`)
  data.value = res.data
  pagination.value = { current: res.data.page, pageSize: res.data.pageSize, total: res.data.total, showSizeChanger: false }
}

function onChange(p: any) { load(p.current) }
async function openAsset(record: EasmAsset) {
  const owner = orgId.value || record.organizationIds?.[0] || ''
  const res = await api<{ data: EasmAsset }>(`/easm/assets/${record.id}${qs({ organizationId: owner })}`)
  detail.value = res.data
  detailOpen.value = true
}
async function openFocus() {
  const id = String(route.query.focus || '')
  if (!id) return
  const owner = String(route.query.organizationId || '')
  const res = await api<{ data: EasmAsset }>(`/easm/assets/${id}${qs({ organizationId: owner })}`)
  detail.value = res.data
  detailOpen.value = true
}
function riskColor(value?: string) { return { critical: 'red', high: 'volcano', medium: 'orange', low: 'blue', info: 'default' }[value || ''] || 'default' }
function filterOrg(input: string, option: any) { return String(option?.children || '').toLowerCase().includes(input.toLowerCase()) }

onMounted(async () => {
  await loadOrganizations()
  await load(1)
  await openFocus().catch(() => undefined)
})
</script>

<style scoped>
.easm-page-shell { display: flex; flex-direction: column; gap: 14px; }
.tab-card { border-radius: 14px; }
.toolbar { display: flex; gap: 10px; flex-wrap: wrap; margin-bottom: 16px; }
.asset-data { max-height: 460px; overflow: auto; background: #0a1526; color: #cfe0ff; padding: 12px; border-radius: 8px; white-space: pre-wrap; }
</style>
