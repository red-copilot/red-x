<template>
  <div>
    <div class="page-title"><div><h1>监控</h1><span class="muted">域名 / IP 周期监控与资产变化</span></div><a-button type="primary" @click="open = true">新建监控</a-button></div>
    <ResourceTable ref="table" endpoint="/monitors" :columns="columns">
      <template #cell="{ column, record }">
        <template v-if="column.dataIndex === 'status'"><a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status }}</a-tag></template>
        <template v-else-if="column.dataIndex === 'nextRunAt'">{{ formatTime(record.nextRunAt) }}</template>
        <template v-else-if="column.dataIndex === 'actions'">
          <a-space>
            <a-button size="small" @click="run(record)">立即运行</a-button>
            <a-button size="small" @click="toggle(record)">{{ record.status === 'active' ? '暂停' : '恢复' }}</a-button>
            <a-popconfirm title="删除监控？" @confirm="remove(record)"><a-button size="small" danger>删除</a-button></a-popconfirm>
          </a-space>
        </template>
        <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
      </template>
    </ResourceTable>
    <a-modal v-model:open="open" title="新建监控" :confirm-loading="saving" @ok="create">
      <a-form layout="vertical">
        <a-form-item label="名称"><a-input v-model:value="form.name" /></a-form-item>
        <a-form-item label="类型"><a-select v-model:value="form.kind"><a-select-option value="domain">域名</a-select-option><a-select-option value="ip">IP</a-select-option><a-select-option value="site_update">站点更新</a-select-option><a-select-option value="wih_update">WIH 更新</a-select-option></a-select></a-form-item>
        <a-form-item label="资产组 ID"><a-input v-model:value="form.scopeId" /></a-form-item>
        <a-form-item label="目标"><a-input v-model:value="form.target" /></a-form-item>
        <a-form-item label="间隔（秒，最小 21600）"><a-input-number v-model:value="form.intervalSec" :min="21600" style="width:100%" /></a-form-item>
        <a-form-item label="策略 ID（可选）"><a-input v-model:value="form.policyId" /></a-form-item>
      </a-form>
    </a-modal>
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
const form = reactive({ name: '', kind: 'domain', scopeId: '', target: '', intervalSec: 21600, policyId: '' })
const columns = [
  { title: '名称', dataIndex: 'name', width: 200 },
  { title: '类型', dataIndex: 'kind', width: 120 },
  { title: '目标', dataIndex: 'target', width: 240 },
  { title: '状态', dataIndex: 'status', width: 100 },
  { title: '下次运行', dataIndex: 'nextRunAt', width: 180 },
  { title: '操作', dataIndex: 'actions', width: 260 },
]
async function create() {
  saving.value = true
  try {
    await api('/monitors', { method: 'POST', body: JSON.stringify(form) })
    message.success('监控已创建')
    open.value = false
    table.value?.reload()
  } catch (e: any) { message.error(e.message) } finally { saving.value = false }
}
async function toggle(record: any) {
  await api(`/monitors/${record.id}/${record.status === 'active' ? 'pause' : 'resume'}`, { method: 'POST', body: JSON.stringify({}) })
  table.value?.reload()
}
async function run(record: any) {
  await api(`/monitors/${record.id}/run`, { method: 'POST', body: JSON.stringify({}) })
  message.success('已触发')
  table.value?.reload()
}
async function remove(record: any) {
  await api(`/monitors/${record.id}`, { method: 'DELETE' })
  table.value?.reload()
}
</script>
