<template>
  <div>
    <div class="page-title"><div><h1>资产</h1><span class="muted">全局规范资产与最近观测</span></div></div>
    <a-tabs v-model:activeKey="kind" @change="() => table?.reload()">
      <a-tab-pane key="domain" tab="域名" />
      <a-tab-pane key="ip" tab="IP" />
      <a-tab-pane key="site" tab="站点" />
      <a-tab-pane key="url" tab="URL" />
      <a-tab-pane key="cert" tab="证书" />
      <a-tab-pane key="service" tab="服务" />
      <a-tab-pane key="cidr" tab="C 段" />
    </a-tabs>
    <ResourceTable ref="table" :endpoint="`/assets/${kind}`" :columns="columns" :params="{}">
      <template #cell="{ column, record }">
        <template v-if="column.dataIndex === 'key'"><span class="mono">{{ record.key }}</span></template>
        <template v-else-if="column.dataIndex === 'title'">{{ record.data?.title || '-' }}</template>
        <template v-else-if="column.dataIndex === 'ip'">{{ record.data?.ip || '-' }}</template>
        <template v-else-if="column.dataIndex === 'status'">{{ record.data?.status || '-' }}</template>
        <template v-else-if="column.dataIndex === 'scanSummary'">{{ scanSummary(record) }}</template>
        <template v-else-if="column.dataIndex === 'lastSeen'">{{ formatTime(record.lastSeen) }}</template>
        <template v-else-if="column.dataIndex === 'data'"><span class="mono">{{ JSON.stringify(record.data) }}</span></template>
        <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
      </template>
    </ResourceTable>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import ResourceTable from '@/components/ResourceTable.vue'
import { formatTime } from '@/api'

const kind = ref('domain')
const table = ref<InstanceType<typeof ResourceTable> | null>(null)
function scanSummary(record: any) {
  const data = record.data || {}
  const values = [
    Array.isArray(data.technologies) ? data.technologies.join(', ') : '',
    data.cdnName,
    data.tls?.version,
    data.osInfo?.name,
    data.product,
    data.version,
  ]
  return values.filter(Boolean).join(' · ') || '-'
}
const columns = computed(() => [
  { title: '资产 Key', dataIndex: 'key', width: 260 },
  { title: '标题', dataIndex: 'title', width: 200 },
  { title: 'IP', dataIndex: 'ip', width: 140 },
  { title: '状态', dataIndex: 'status', width: 90 },
  { title: '扫描信息', dataIndex: 'scanSummary', width: 260 },
  { title: '最近发现', dataIndex: 'lastSeen', width: 180 },
  { title: '数据', dataIndex: 'data', width: 420 },
])
</script>
