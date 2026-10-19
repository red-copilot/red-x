<template>
  <div>
    <div class="page-title"><div><h1>GitHub</h1><span class="muted">代码搜索、泄漏监控与结果去重</span></div></div>
    <a-tabs v-model:activeKey="tab" @change="() => table?.reload()">
      <a-tab-pane key="tasks" tab="搜索任务" />
      <a-tab-pane key="monitors" tab="监控任务" />
      <a-tab-pane key="findings" tab="结果" />
    </a-tabs>
    <ResourceTable ref="table" :endpoint="endpoint" :columns="columns">
      <template #toolbar>
        <a-button v-if="tab !== 'findings'" type="primary" @click="open = true">新建{{ tab === 'tasks' ? '搜索任务' : '监控' }}</a-button>
      </template>
      <template #cell="{ column, record }">
        <template v-if="column.dataIndex === 'status'"><a-tag>{{ record.status }}</a-tag></template>
        <template v-else-if="column.dataIndex === 'createdAt'">{{ formatTime(record.createdAt) }}</template>
        <template v-else-if="column.dataIndex === 'repo'"><a :href="record.htmlUrl" target="_blank">{{ record.repo }}</a></template>
        <template v-else-if="column.dataIndex === 'actions'"><a-popconfirm title="删除监控？" @confirm="remove(record)"><a-button size="small" danger>删除</a-button></a-popconfirm></template>
        <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
      </template>
    </ResourceTable>
    <a-modal v-model:open="open" :title="tab === 'tasks' ? 'GitHub 代码搜索' : 'GitHub 监控'" :confirm-loading="saving" @ok="create">
      <a-form layout="vertical">
        <a-form-item label="关键字"><a-input v-model:value="form.keyword" placeholder="example.com password" /></a-form-item>
        <a-form-item v-if="tab === 'monitors'" label="名称"><a-input v-model:value="form.name" /></a-form-item>
        <a-form-item v-if="tab === 'monitors'" label="间隔（秒）"><a-input-number v-model:value="form.intervalSec" :min="3600" style="width:100%" /></a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import ResourceTable from '@/components/ResourceTable.vue'
import { api, formatTime } from '@/api'

const tab = ref('tasks')
const table = ref<InstanceType<typeof ResourceTable> | null>(null)
const open = ref(false)
const saving = ref(false)
const form = reactive({ keyword: '', name: '', intervalSec: 21600 })
const endpoint = computed(() => `/github/${tab.value}`)
const columns = computed(() => {
  if (tab.value === 'findings') {
    return [
      { title: '仓库', dataIndex: 'repo', width: 240 },
      { title: '文件', dataIndex: 'path', width: 260 },
      { title: '关键字', dataIndex: 'keyword', width: 160 },
      { title: 'Hash', dataIndex: 'hashMd5', width: 240 },
      { title: '时间', dataIndex: 'createdAt', width: 180 },
    ]
  }
  if (tab.value === 'monitors') {
    return [
      { title: '名称', dataIndex: 'name', width: 200 },
      { title: '关键字', dataIndex: 'keyword', width: 240 },
      { title: '状态', dataIndex: 'status', width: 100 },
      { title: '间隔', dataIndex: 'intervalSec', width: 100 },
      { title: '操作', dataIndex: 'actions', width: 100 },
    ]
  }
  return [
    { title: '关键字', dataIndex: 'keyword', width: 260 },
    { title: '状态', dataIndex: 'status', width: 120 },
    { title: '结果数', dataIndex: 'resultCount', width: 100 },
    { title: '创建时间', dataIndex: 'createdAt', width: 180 },
  ]
})
async function create() {
  saving.value = true
  try {
    if (tab.value === 'tasks') await api('/github/tasks', { method: 'POST', body: JSON.stringify({ keyword: form.keyword }) })
    else await api('/github/monitors', { method: 'POST', body: JSON.stringify(form) })
    message.success('已创建')
    open.value = false
    table.value?.reload()
  } catch (e: any) { message.error(e.message) } finally { saving.value = false }
}
async function remove(record: any) {
  await api(`/github/monitors/${record.id}`, { method: 'DELETE' })
  table.value?.reload()
}
</script>
