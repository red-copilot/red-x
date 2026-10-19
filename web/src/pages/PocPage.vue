<template>
  <div>
    <div class="page-title"><div><h1>PoC 插件</h1><span class="muted">Go 原生 NPoC 兼容引擎 · 40 PoC / 28 Sniffer / 28 Brute</span></div><a-button @click="sync">同步元数据</a-button></div>
    <a-card :bordered="false" style="border-radius:14px">
      <div class="table-toolbar">
        <a-select v-model:value="kind" allow-clear placeholder="类型" style="width:180px">
          <a-select-option value="poc">PoC</a-select-option>
          <a-select-option value="sniffer">协议识别</a-select-option>
          <a-select-option value="brute">弱口令</a-select-option>
        </a-select>
        <a-input-search v-model:value="search" placeholder="搜索插件" style="max-width:320px" />
      </div>
      <a-table :data-source="filtered" :columns="columns" row-key="name" size="middle" :loading="loading">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'type'"><a-tag :color="record.type === 'poc' ? 'blue' : record.type === 'brute' ? 'red' : 'green'">{{ record.type }}</a-tag></template>
          <template v-else-if="column.dataIndex === 'schemes'">{{ (record.schemes || []).join(', ') }}</template>
          <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { api } from '@/api'

const items = ref<any[]>([])
const loading = ref(false)
const kind = ref<string>()
const search = ref('')
const columns = [
  { title: '插件名', dataIndex: 'name', width: 260 },
  { title: '类型', dataIndex: 'type', width: 100 },
  { title: '应用', dataIndex: 'appName', width: 160 },
  { title: '漏洞 / 能力', dataIndex: 'vulName', width: 280 },
  { title: '协议', dataIndex: 'schemes', width: 180 },
]
const filtered = computed(() => items.value.filter((x) => {
  const matchesKind = !kind.value || x.type === kind.value
  const matchesSearch = !search.value || `${x.name} ${x.appName} ${x.vulName}`.toLowerCase().includes(search.value.toLowerCase())
  return matchesKind && matchesSearch
}))
async function load() {
  loading.value = true
  try {
    const res = await api<{ data: { items: any[] } }>('/poc/plugins')
    items.value = res.data.items || []
  } finally { loading.value = false }
}
async function sync() {
  const res = await api<{ data: { synced: number } }>('/poc/sync', { method: 'POST', body: JSON.stringify({}) })
  message.success(`已同步 ${res.data.synced} 个插件`)
}
onMounted(load)
</script>
