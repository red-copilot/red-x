<template>
  <div>
    <div class="page-title"><div><h1>资产组</h1><span class="muted">按域名或 IP 范围聚合资产</span></div><a-button type="primary" @click="open = true">新建资产组</a-button></div>
    <ResourceTable ref="table" endpoint="/scopes" :columns="columns">
      <template #cell="{ column, record }">
        <template v-if="column.dataIndex === 'targets'">{{ (record.targets || []).join(', ') }}</template>
        <template v-else-if="column.dataIndex === 'createdAt'">{{ formatTime(record.createdAt) }}</template>
        <template v-else-if="column.dataIndex === 'actions'">
          <a-space><a-button size="small" @click="viewAssets(record)">资产</a-button><a-popconfirm title="删除资产组？" @confirm="remove(record)"><a-button size="small" danger>删除</a-button></a-popconfirm></a-space>
        </template>
        <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
      </template>
    </ResourceTable>
    <a-modal v-model:open="open" title="新建资产组" :confirm-loading="saving" @ok="create">
      <a-form layout="vertical">
        <a-form-item label="名称"><a-input v-model:value="form.name" /></a-form-item>
        <a-form-item label="类型"><a-select v-model:value="form.type"><a-select-option value="domain">域名</a-select-option><a-select-option value="ip">IP / CIDR</a-select-option></a-select></a-form-item>
        <a-form-item label="范围（逗号分隔）"><a-textarea v-model:value="form.targets" :rows="3" /></a-form-item>
      </a-form>
    </a-modal>
    <a-drawer v-model:open="assetOpen" :title="`资产组资产 · ${currentScope?.name || ''}`" width="900">
      <a-table :data-source="scopeAssets" :columns="assetColumns" row-key="asset.key" size="small" :loading="assetLoading">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'key'"><span class="mono">{{ record.asset?.key }}</span></template>
          <template v-else-if="column.dataIndex === 'lastSeen'">{{ formatTime(record.lastSeen) }}</template>
          <template v-else-if="column.dataIndex === 'tags'">{{ (record.tags || []).join(', ') }}</template>
        </template>
      </a-table>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import ResourceTable from '@/components/ResourceTable.vue'
import { api, formatTime } from '@/api'

const table = ref<InstanceType<typeof ResourceTable> | null>(null)
const open = ref(false)
const saving = ref(false)
const assetOpen = ref(false)
const assetLoading = ref(false)
const currentScope = ref<any>(null)
const scopeAssets = ref<any[]>([])
const form = reactive({ name: '', type: 'domain', targets: '' })
const columns = [
  { title: '名称', dataIndex: 'name', width: 220 },
  { title: '类型', dataIndex: 'type', width: 100 },
  { title: '范围', dataIndex: 'targets', width: 380 },
  { title: '创建时间', dataIndex: 'createdAt', width: 180 },
  { title: '操作', dataIndex: 'actions', width: 150 },
]
const assetColumns = [
  { title: '资产', dataIndex: 'key', width: 240 },
  { title: '类型', dataIndex: 'kind', width: 100 },
  { title: '最近发现', dataIndex: 'lastSeen', width: 180 },
  { title: '标签', dataIndex: 'tags', width: 200 },
]

async function create() {
  saving.value = true
  try {
    await api('/scopes', { method: 'POST', body: JSON.stringify({ ...form, targets: form.targets.split(/[\s,]+/).filter(Boolean) }) })
    message.success('资产组已创建')
    open.value = false
    table.value?.reload()
  } catch (e: any) { message.error(e.message) } finally { saving.value = false }
}
async function remove(record: any) {
  await api(`/scopes/${record.id}`, { method: 'DELETE' })
  table.value?.reload()
}
async function viewAssets(record: any) {
  currentScope.value = record
  assetOpen.value = true
  assetLoading.value = true
  try {
    const res = await api<{ data: { items: any[] } }>(`/scopes/${record.id}/assets?pageSize=200`)
    scopeAssets.value = res.data.items || []
  } finally { assetLoading.value = false }
}
</script>
