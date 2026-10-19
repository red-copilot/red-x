<template>
  <div>
    <div class="page-title">
      <div><h1>任务</h1><span class="muted">域名、IP、风险巡航与资产更新任务</span></div>
      <a-button type="primary" @click="openCreate">新建任务</a-button>
    </div>
    <ResourceTable ref="table" endpoint="/tasks" :columns="columns" :params="filters">
      <template #toolbar>
        <a-select v-model:value="filters.status" placeholder="状态" allow-clear style="width: 140px" @change="table?.reload()">
          <a-select-option v-for="s in statuses" :key="s" :value="s">{{ s }}</a-select-option>
        </a-select>
        <a-select v-model:value="filters.kind" placeholder="类型" allow-clear style="width: 150px" @change="table?.reload()">
          <a-select-option value="domain">域名</a-select-option>
          <a-select-option value="ip">IP</a-select-option>
          <a-select-option value="risk_cruising">风险巡航</a-select-option>
          <a-select-option value="asset_site_update">站点更新</a-select-option>
        </a-select>
      </template>
      <template #cell="{ column, record }">
        <template v-if="column.dataIndex === 'name'">
          <a @click="$router.push(`/tasks/${record.id}`)">{{ record.name }}</a>
        </template>
        <template v-else-if="column.dataIndex === 'status'">
          <a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag>
        </template>
        <template v-else-if="column.dataIndex === 'progress'">
          <a-progress :percent="record.progress || 0" size="small" />
        </template>
        <template v-else-if="column.dataIndex === 'createdAt'">{{ formatTime(record.createdAt) }}</template>
        <template v-else-if="column.dataIndex === 'actions'">
          <a-space>
            <a-button size="small" @click="$router.push(`/tasks/${record.id}`)">详情</a-button>
            <a-button v-if="!terminal(record.status)" size="small" danger @click="cancel(record)">停止</a-button>
          </a-space>
        </template>
        <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
      </template>
    </ResourceTable>

    <a-modal v-model:open="createOpen" title="新建任务" width="720px" :confirm-loading="creating" @ok="create">
      <a-form layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12"><a-form-item label="任务名称"><a-input v-model:value="form.name" placeholder="可选" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="任务类型">
            <a-select v-model:value="form.kind">
              <a-select-option value="domain">域名资产发现</a-select-option>
              <a-select-option value="ip">IP 资产发现</a-select-option>
              <a-select-option value="risk_cruising">风险巡航</a-select-option>
            </a-select>
          </a-form-item></a-col>
        </a-row>
        <a-form-item label="目标"><a-textarea v-model:value="form.target" :rows="3" placeholder="example.com / 10.0.0.0/24 / https://site.example" /></a-form-item>
        <a-row :gutter="16">
          <a-col :span="12"><a-form-item label="策略 ID（可选）"><a-input v-model:value="form.policyId" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="资产组 ID（可选）"><a-input v-model:value="form.scopeId" /></a-form-item></a-col>
        </a-row>
        <a-form-item label="扫描选项">
          <a-space wrap>
            <a-checkbox v-model:checked="form.options.domainBrute">域名爆破</a-checkbox>
            <a-checkbox v-model:checked="form.options.portScan">端口扫描</a-checkbox>
            <a-checkbox v-model:checked="form.options.serviceDetection">服务识别</a-checkbox>
            <a-checkbox v-model:checked="form.options.siteIdentify">指纹识别</a-checkbox>
            <a-checkbox v-model:checked="form.options.siteCapture">站点截图</a-checkbox>
            <a-checkbox v-model:checked="form.options.siteSpider">站点爬虫</a-checkbox>
            <a-checkbox v-model:checked="form.options.fileLeak">文件泄漏</a-checkbox>
            <a-checkbox v-model:checked="form.options.webInfoHunter">WIH</a-checkbox>
          </a-space>
        </a-form-item>
        <a-form-item label="端口范围">
          <a-select v-model:value="form.options.portScanType">
            <a-select-option value="test">测试端口</a-select-option>
            <a-select-option value="top100">TOP 100</a-select-option>
            <a-select-option value="top1000">TOP 1000</a-select-option>
            <a-select-option value="all">全部端口</a-select-option>
            <a-select-option value="custom">自定义</a-select-option>
          </a-select>
        </a-form-item>
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
const createOpen = ref(false)
const creating = ref(false)
const filters = reactive({ status: undefined as string | undefined, kind: undefined as string | undefined })
const statuses = ['queued', 'running', 'succeeded', 'partial', 'failed', 'canceled']
const columns = [
  { title: '任务', dataIndex: 'name', width: 220 },
  { title: '目标', dataIndex: 'target', width: 220 },
  { title: '类型', dataIndex: 'kind', width: 120 },
  { title: '状态', dataIndex: 'status', width: 110 },
  { title: '进度', dataIndex: 'progress', width: 150 },
  { title: '创建时间', dataIndex: 'createdAt', width: 180 },
  { title: '操作', dataIndex: 'actions', width: 150 },
]
const form = reactive({
  name: '',
  kind: 'domain',
  target: '',
  policyId: '',
  scopeId: '',
  options: {
    domainBrute: true,
    portScan: true,
    portScanType: 'test',
    serviceDetection: false,
    siteIdentify: true,
    siteCapture: false,
    siteSpider: false,
    fileLeak: false,
    webInfoHunter: false,
  },
})

function statusColor(status: string) {
  return { queued: 'default', running: 'processing', succeeded: 'green', partial: 'orange', failed: 'red', canceled: 'default' }[status] || 'default'
}
function terminal(status: string) {
  return ['succeeded', 'partial', 'failed', 'canceled'].includes(status)
}
function openCreate() { createOpen.value = true }
async function create() {
  if (!form.target.trim()) { message.warning('请输入目标'); return }
  creating.value = true
  try {
    await api('/tasks', { method: 'POST', body: JSON.stringify(form) })
    message.success('任务已创建')
    createOpen.value = false
    table.value?.reload()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    creating.value = false
  }
}
async function cancel(record: any) {
  await api(`/tasks/${record.id}/cancel`, { method: 'POST', body: JSON.stringify({}) })
  message.success('已取消')
  table.value?.reload()
}
</script>
