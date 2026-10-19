<template>
  <div>
    <div class="page-title"><div><h1>策略</h1><span class="muted">扫描能力与插件组合</span></div><a-button type="primary" @click="open = true">新建策略</a-button></div>
    <ResourceTable ref="table" endpoint="/policies" :columns="columns">
      <template #cell="{ column, record }">
        <template v-if="column.dataIndex === 'config'"><span class="mono">{{ policySummary(record.config) }}</span></template>
        <template v-else-if="column.dataIndex === 'actions'"><a-popconfirm title="删除策略？" @confirm="remove(record)"><a-button size="small" danger>删除</a-button></a-popconfirm></template>
        <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
      </template>
    </ResourceTable>
    <a-modal v-model:open="open" title="新建策略" width="640px" :confirm-loading="saving" @ok="create">
      <a-form layout="vertical">
        <a-form-item label="名称"><a-input v-model:value="form.name" /></a-form-item>
        <a-form-item label="描述"><a-input v-model:value="form.description" /></a-form-item>
        <a-form-item label="扫描选项">
          <a-space wrap>
            <a-checkbox v-model:checked="form.config.domain.domainBrute">域名爆破</a-checkbox>
            <a-checkbox v-model:checked="form.config.ip.portScan">端口扫描</a-checkbox>
            <a-checkbox v-model:checked="form.config.ip.serviceDetection">服务识别</a-checkbox>
            <a-checkbox v-model:checked="form.config.site.siteIdentify">指纹识别</a-checkbox>
            <a-checkbox v-model:checked="form.config.site.fileLeak">文件泄漏</a-checkbox>
            <a-checkbox v-model:checked="form.config.site.webInfoHunter">WIH</a-checkbox>
          </a-space>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import ResourceTable from '@/components/ResourceTable.vue'
import { api } from '@/api'

const table = ref<InstanceType<typeof ResourceTable> | null>(null)
const open = ref(false)
const saving = ref(false)
const columns = [
  { title: '名称', dataIndex: 'name', width: 200 },
  { title: '描述', dataIndex: 'description', width: 240 },
  { title: '配置', dataIndex: 'config', width: 460 },
  { title: '操作', dataIndex: 'actions', width: 100 },
]
const form = reactive({
  name: '',
  description: '',
  config: {
    domain: { domainBrute: true, domainBruteType: 'test', altDns: false, arlSearch: true, dnsQueryPlugin: false },
    ip: { portScan: true, portScanType: 'test', serviceDetection: false, osDetection: false, sslCert: false, skipScanCdnIp: true },
    site: { siteIdentify: true, siteCapture: false, searchEngines: false, siteSpider: false, fileLeak: false, webInfoHunter: false, nucleiScan: false },
    poc: [],
    brute: [],
  },
})
function policySummary(config: any) {
  if (!config) return '-'
  return Object.entries({ ...config.domain, ...config.ip, ...config.site }).filter(([, v]) => v === true).map(([k]) => k).join(', ') || '无'
}
async function create() {
  saving.value = true
  try {
    await api('/policies', { method: 'POST', body: JSON.stringify(form) })
    message.success('策略已创建')
    open.value = false
    table.value?.reload()
  } catch (e: any) { message.error(e.message) } finally { saving.value = false }
}
async function remove(record: any) {
  await api(`/policies/${record.id}`, { method: 'DELETE' })
  table.value?.reload()
}
</script>
