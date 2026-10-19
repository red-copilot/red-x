<template>
  <div class="easm-page-shell">
    <div class="page-title">
      <div><h1>EASM 任务</h1><span class="muted">组织级扫描任务及运行状态</span></div>
    </div>
    <a-card :bordered="false" class="tab-card">
      <div class="toolbar">
        <a-input-search v-model:value="query" placeholder="搜索任务或目标" style="max-width: 320px" @search="load(1)" />
        <a-select v-model:value="orgId" show-search allow-clear placeholder="全部组织" style="width: 240px" :filter-option="filterOrg" @change="load(1)">
          <a-select-option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.canonicalName }}</a-select-option>
        </a-select>
        <a-select v-model:value="status" placeholder="状态" allow-clear style="width: 140px" @change="load(1)">
          <a-select-option v-for="s in ['queued', 'running', 'succeeded', 'partial', 'failed', 'canceled']" :key="s" :value="s">{{ s }}</a-select-option>
        </a-select>
        <a-button @click="load(page)">刷新</a-button>
      </div>
      <a-table :data-source="items" :columns="columns" row-key="id" size="middle" :pagination="pagination" @change="onChange">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'name'"><a @click="$router.push(`/tasks/${record.id}`)">{{ record.name }}</a></template>
          <template v-else-if="column.dataIndex === 'status'"><a-tag>{{ record.status }}</a-tag></template>
          <template v-else-if="column.dataIndex === 'progress'"><a-progress :percent="record.progress || 0" size="small" /></template>
          <template v-else-if="column.dataIndex === 'createdAt'">{{ formatTime(record.createdAt) }}</template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, formatTime, qs } from '@/api'
import type { EasmTask, Organization } from '@/components/easm/types'

const organizations = ref<Organization[]>([])
const orgId = ref<string | undefined>()
const status = ref<string | undefined>()
const query = ref('')
const page = ref(1)
const pageSize = ref(20)
const items = ref<EasmTask[]>([])
const pagination = ref({ current: 1, pageSize: 20, total: 0, showSizeChanger: false })
const columns = [
  { title: '任务', dataIndex: 'name', width: 300 },
  { title: '主域', dataIndex: 'rootDomain', width: 220 },
  { title: '状态', dataIndex: 'status', width: 110 },
  { title: '进度', dataIndex: 'progress', width: 180 },
  { title: '创建时间', dataIndex: 'createdAt', width: 180 },
]

async function loadOrganizations() {
  const res = await api<{ data: { items: Organization[] } }>('/organizations?limit=200')
  organizations.value = res.data.items || []
}

async function load(next = page.value) {
  page.value = next
  const res = await api<{ data: { items: EasmTask[]; total: number; page: number; pageSize: number } }>(`/easm/tasks${qs({
    organizationId: orgId.value, status: status.value, q: query.value, page: page.value, pageSize: pageSize.value,
  })}`)
  items.value = res.data.items || []
  pagination.value = { current: res.data.page || 1, pageSize: res.data.pageSize || 20, total: res.data.total || 0, showSizeChanger: false }
}

function onChange(p: any) { load(p.current) }
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
