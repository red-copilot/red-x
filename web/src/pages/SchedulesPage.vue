<template>
  <div>
    <div class="page-title"><div><h1>计划任务</h1><span class="muted">Cron 周期任务与一次性任务</span></div><a-button type="primary" @click="open = true">新建计划</a-button></div>
    <ResourceTable ref="table" endpoint="/schedules" :columns="columns">
      <template #cell="{ column, record }">
        <template v-if="column.dataIndex === 'nextRunAt'">{{ formatTime(record.nextRunAt) }}</template>
        <template v-else-if="column.dataIndex === 'actions'"><a-popconfirm title="删除计划？" @confirm="remove(record)"><a-button size="small" danger>删除</a-button></a-popconfirm></template>
        <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
      </template>
    </ResourceTable>
    <a-modal v-model:open="open" title="新建计划任务" :confirm-loading="saving" @ok="create">
      <a-form layout="vertical">
        <a-form-item label="名称"><a-input v-model:value="form.name" /></a-form-item>
        <a-form-item label="类型"><a-select v-model:value="form.kind"><a-select-option value="recurrent_scan">周期扫描</a-select-option><a-select-option value="future_scan">一次性任务</a-select-option></a-select></a-form-item>
        <a-form-item label="任务类型"><a-select v-model:value="form.taskKind"><a-select-option value="domain">域名</a-select-option><a-select-option value="ip">IP</a-select-option></a-select></a-form-item>
        <a-form-item label="目标"><a-input v-model:value="form.target" /></a-form-item>
        <a-form-item label="策略 ID"><a-input v-model:value="form.policyId" /></a-form-item>
        <a-form-item v-if="form.kind === 'recurrent_scan'" label="Cron（5 段）"><a-input v-model:value="form.cron" placeholder="0 2 * * *" /></a-form-item>
        <a-form-item v-else label="开始时间"><a-input v-model:value="form.startAt" placeholder="2026-01-01T02:00:00Z" /></a-form-item>
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
const form = reactive({ name: '', kind: 'recurrent_scan', taskKind: 'domain', target: '', policyId: '', cron: '0 2 * * *', startAt: '' })
const columns = [
  { title: '名称', dataIndex: 'name', width: 220 },
  { title: '类型', dataIndex: 'kind', width: 140 },
  { title: '目标', dataIndex: 'target', width: 240 },
  { title: 'Cron', dataIndex: 'cron', width: 130 },
  { title: '状态', dataIndex: 'status', width: 100 },
  { title: '下次运行', dataIndex: 'nextRunAt', width: 180 },
  { title: '操作', dataIndex: 'actions', width: 100 },
]
async function create() {
  saving.value = true
  try {
    const body: any = { ...form }
    if (form.kind === 'future_scan' && form.startAt) body.startAt = new Date(form.startAt).toISOString()
    await api('/schedules', { method: 'POST', body: JSON.stringify(body) })
    message.success('计划任务已创建')
    open.value = false
    table.value?.reload()
  } catch (e: any) { message.error(e.message) } finally { saving.value = false }
}
async function remove(record: any) {
  await api(`/schedules/${record.id}`, { method: 'DELETE' })
  table.value?.reload()
}
</script>
