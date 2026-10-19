<template>
  <div>
    <div class="page-title"><div><h1>发现</h1><span class="muted">漏洞、nuclei、WIH、文件泄漏与协议识别</span></div></div>
    <a-tabs v-model:activeKey="kind" @change="() => table?.reload()">
      <a-tab-pane key="vuln" tab="漏洞" />
      <a-tab-pane key="nuclei" tab="nuclei" />
      <a-tab-pane key="wih" tab="WIH" />
      <a-tab-pane key="file_leak" tab="文件泄漏" />
      <a-tab-pane key="protocol" tab="协议识别" />
    </a-tabs>
    <ResourceTable ref="table" :endpoint="`/findings/${kind}`" :columns="columns">
      <template #cell="{ column, record }">
        <template v-if="column.dataIndex === 'severity'"><a-tag :color="severityColor(record.severity)">{{ record.severity }}</a-tag></template>
        <template v-else-if="column.dataIndex === 'createdAt'">{{ formatTime(record.createdAt) }}</template>
        <template v-else-if="column.dataIndex === 'data'"><span class="mono">{{ JSON.stringify(record.data) }}</span></template>
        <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
      </template>
    </ResourceTable>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ResourceTable from '@/components/ResourceTable.vue'
import { formatTime } from '@/api'

const kind = ref('vuln')
const table = ref<InstanceType<typeof ResourceTable> | null>(null)
const columns = [
  { title: '标题', dataIndex: 'title', width: 260 },
  { title: '目标', dataIndex: 'target', width: 260 },
  { title: '插件', dataIndex: 'pluginName', width: 180 },
  { title: '级别', dataIndex: 'severity', width: 100 },
  { title: '时间', dataIndex: 'createdAt', width: 180 },
  { title: '数据', dataIndex: 'data', width: 420 },
]
function severityColor(s: string) {
  return { critical: 'red', high: 'volcano', medium: 'orange', low: 'blue', info: 'default' }[s] || 'default'
}
</script>
