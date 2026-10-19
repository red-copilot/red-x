<template>
  <div>
    <div class="page-title"><div><h1>指纹库</h1><span class="muted">布尔表达式规则 · body / header / title / icon_hash</span></div><a-button type="primary" @click="open = true">新增指纹</a-button></div>
    <a-card :bordered="false" style="border-radius:14px">
      <div class="table-toolbar">
        <a-input-search v-model:value="search" placeholder="搜索指纹" style="max-width:320px" />
        <a-button @click="load">刷新</a-button>
        <a-button @click="importOpen = true">批量导入</a-button>
      </div>
      <a-table :data-source="filtered" :columns="columns" row-key="id" size="middle" :loading="loading">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'rule'"><span class="mono">{{ record.rule }}</span></template>
          <template v-else-if="column.dataIndex === 'actions'"><a-popconfirm title="删除指纹？" @confirm="remove(record)"><a-button size="small" danger>删除</a-button></a-popconfirm></template>
          <template v-else>{{ record[column.dataIndex] }}</template>
        </template>
      </a-table>
    </a-card>
    <a-modal v-model:open="open" title="新增指纹" :confirm-loading="saving" @ok="create">
      <a-form layout="vertical">
        <a-form-item label="名称"><a-input v-model:value="form.name" /></a-form-item>
        <a-form-item label="规则"><a-textarea v-model:value="form.rule" :rows="4" placeholder='body="example" || title="Example"' /></a-form-item>
      </a-form>
    </a-modal>
    <a-modal v-model:open="importOpen" title="批量导入 YAML/JSON" :confirm-loading="saving" @ok="importRules">
      <a-textarea v-model:value="importText" :rows="12" placeholder='- name: Example\n  rule: body="example"' />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { api } from '@/api'

const items = ref<any[]>([])
const loading = ref(false)
const open = ref(false)
const importOpen = ref(false)
const saving = ref(false)
const search = ref('')
const importText = ref('')
const form = reactive({ name: '', rule: '' })
const columns = [
  { title: '名称', dataIndex: 'name', width: 220 },
  { title: '规则', dataIndex: 'rule', width: 620 },
  { title: '操作', dataIndex: 'actions', width: 100 },
]
const filtered = computed(() => items.value.filter((x) => !search.value || `${x.name} ${x.rule}`.toLowerCase().includes(search.value.toLowerCase())))
async function load() {
  loading.value = true
  try {
    const res = await api<{ data: { items: any[] } }>('/fingerprints')
    items.value = res.data.items || []
  } finally { loading.value = false }
}
async function create() {
  saving.value = true
  try {
    await api('/fingerprints', { method: 'POST', body: JSON.stringify(form) })
    message.success('指纹已创建')
    open.value = false
    load()
  } catch (e: any) { message.error(e.message) } finally { saving.value = false }
}
async function remove(record: any) {
  await api(`/fingerprints/${record.id}`, { method: 'DELETE' })
  load()
}
async function importRules() {
  saving.value = true
  try {
    const res = await api<{ data: { successCount: number; errorCount: number } }>('/fingerprints/import', {
      method: 'POST',
      body: importText.value,
      headers: { 'Content-Type': 'application/x-yaml' },
    })
    message.success(`导入 ${res.data.successCount} 条，失败 ${res.data.errorCount} 条`)
    importOpen.value = false
    load()
  } catch (e: any) { message.error(e.message) } finally { saving.value = false }
}
onMounted(load)
</script>
