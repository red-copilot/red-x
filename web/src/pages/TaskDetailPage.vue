<template>
  <div v-if="task">
    <div class="page-title">
      <div><h1>{{ task.name }}</h1><span class="muted">{{ task.target }} · {{ task.kind }}</span></div>
      <a-space>
        <a-tag :color="statusColor(task.status)">{{ task.status }}</a-tag>
        <a-button v-if="!terminal(task.status)" danger @click="cancel">停止</a-button>
        <a-button @click="exportReport">导出</a-button>
      </a-space>
    </div>
    <a-row :gutter="[16,16]">
      <a-col :xs="24" :lg="15">
        <a-card title="执行步骤" :bordered="false" style="border-radius:14px">
          <a-timeline>
            <a-timeline-item v-for="step in steps" :key="step.id" :color="stepColor(step.status)">
              <div><strong>{{ step.kind }}</strong> <a-tag style="margin-left:8px">{{ step.status }}</a-tag></div>
              <div class="muted">{{ step.message || step.error || '-' }}</div>
              <a-progress :percent="step.progress || 0" size="small" />
            </a-timeline-item>
          </a-timeline>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="9">
        <a-card title="任务信息" :bordered="false" style="border-radius:14px">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="状态">{{ task.status }}</a-descriptions-item>
            <a-descriptions-item v-if="task.origin" label="任务来源">{{ task.origin }}</a-descriptions-item>
            <a-descriptions-item v-if="task.rootDomain" label="EASM 主域">{{ task.rootDomain }}</a-descriptions-item>
            <a-descriptions-item v-if="task.organizationId" label="所属组织">
              <a @click="$router.push(`/easm/organizations/${task.organizationId}`)">{{ task.organizationId }}</a>
            </a-descriptions-item>
            <a-descriptions-item label="进度">{{ task.progress }}%</a-descriptions-item>
            <a-descriptions-item label="创建时间">{{ formatTime(task.createdAt) }}</a-descriptions-item>
            <a-descriptions-item label="完成时间">{{ formatTime(task.finishedAt) }}</a-descriptions-item>
            <a-descriptions-item label="错误">{{ task.error || '-' }}</a-descriptions-item>
          </a-descriptions>
          <a-alert v-if="task.warnings?.length" type="warning" :message="task.warnings.join('；')" style="margin-top:12px" />
        </a-card>
      </a-col>
    </a-row>
    <a-card :bordered="false" style="margin-top:16px;border-radius:14px">
      <a-tabs v-model:activeKey="tab" @change="loadTab">
        <a-tab-pane key="domains" tab="域名" />
        <a-tab-pane key="ips" tab="IP" />
        <a-tab-pane key="sites" tab="站点" />
        <a-tab-pane key="vulns" tab="漏洞" />
        <a-tab-pane key="wih" tab="WIH" />
        <a-tab-pane key="events" tab="事件流" />
      </a-tabs>
      <a-table v-if="tab !== 'events'" :data-source="rows" :columns="columns" :loading="loading" row-key="id" size="small" :scroll="{ x: 900 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'data'"><span class="mono">{{ JSON.stringify(record.data || record.snapshot) }}</span></template>
          <template v-else-if="column.dataIndex === 'title'">{{ record.title || record.key }}</template>
          <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
        </template>
      </a-table>
      <div v-else>
        <div v-for="event in events" :key="event.id" class="event-row">
          <a-tag>{{ event.type }}</a-tag>
          <span>{{ event.message }}</span>
          <span class="muted">{{ formatTime(event.createdAt) }}</span>
        </div>
      </div>
    </a-card>
  </div>
  <a-spin v-else size="large" style="width:100%;margin-top:20vh" />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useRoute } from 'vue-router'
import { api, formatTime } from '@/api'

const route = useRoute()
const task = ref<any>(null)
const steps = ref<any[]>([])
const events = ref<any[]>([])
const rows = ref<any[]>([])
const tab = ref('domains')
const loading = ref(false)
let source: EventSource | null = null
let refreshTimer: ReturnType<typeof setTimeout> | undefined

const columns = [
  { title: '名称 / 目标', dataIndex: 'title', width: 260 },
  { title: '类型', dataIndex: 'kind', width: 100 },
  { title: '来源', dataIndex: 'source', width: 140 },
  { title: '数据', dataIndex: 'data', width: 480 },
]

function statusColor(status: string) {
  return { queued: 'default', running: 'processing', succeeded: 'green', partial: 'orange', failed: 'red', canceled: 'default' }[status] || 'default'
}
function stepColor(status: string) {
  return { pending: 'gray', queued: 'blue', running: 'blue', succeeded: 'green', failed: 'red', canceled: 'gray', skipped: 'gray' }[status] || 'gray'
}
function terminal(status: string) { return ['succeeded', 'partial', 'failed', 'canceled'].includes(status) }

async function load() {
  await refreshTask()
  connectEvents()
}
async function refreshTask() {
  const res = await api<{ data: { task: any; steps: any[] } }>(`/tasks/${route.params.id}`)
  task.value = res.data.task
  steps.value = res.data.steps || []
  await loadTab()
}
async function loadTab() {
  if (!task.value) return
  loading.value = true
  try {
    if (tab.value === 'events') {
      return
    }
    if (['vulns', 'wih'].includes(tab.value)) {
      const res = await api<{ data: { items: any[] } }>(`/tasks/${route.params.id}/findings?kind=${tab.value === 'vulns' ? 'vuln' : 'wih'}&pageSize=100`)
      rows.value = res.data.items || []
    } else {
      const kind = tab.value === 'domains' ? 'domain' : tab.value === 'ips' ? 'ip' : 'site'
      const res = await api<{ data: { items: any[] } }>(`/tasks/${route.params.id}/observations?kind=${kind}&pageSize=100`)
      rows.value = (res.data.items || []).map((x: any) => ({ ...x, title: x.key, data: x.snapshot }))
    }
  } finally {
    loading.value = false
  }
}
function connectEvents() {
  source?.close()
  source = new EventSource(`/api/v1/tasks/${route.params.id}/events`)
  const onEvent = (e: MessageEvent) => {
    try {
      const ev = JSON.parse(e.data)
      events.value.push(ev)
      if (ev.type === 'task.status' || ev.type === 'step.status' || ev.type === 'task.warning') scheduleRefresh()
    } catch {}
  }
  source.onmessage = onEvent
  ;['task.status', 'step.status', 'task.warning', 'asset.new', 'finding.new'].forEach((name) => source?.addEventListener(name, onEvent as EventListener))
}
function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => {
    refreshTimer = undefined
    refreshTask()
  }, 100)
}
async function cancel() {
  await api(`/tasks/${route.params.id}/cancel`, { method: 'POST', body: JSON.stringify({}) })
  message.success('已停止')
  await refreshTask()
}
async function exportReport() {
  const res = await api<{ data: { downloadUrl: string } }>('/exports', { method: 'POST', body: JSON.stringify({ taskId: task.value.id, format: 'xlsx' }) })
  window.open(res.data.downloadUrl, '_blank')
}

onMounted(load)
onBeforeUnmount(() => {
  source?.close()
  if (refreshTimer) clearTimeout(refreshTimer)
})
</script>

<style scoped>
.event-row { display: flex; gap: 12px; align-items: center; padding: 8px 0; border-bottom: 1px solid #f0f2f5; }
</style>
