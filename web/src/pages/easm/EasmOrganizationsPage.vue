<template>
  <div class="easm-page-shell">
    <div class="page-title">
      <div>
        <h1>EASM 组织</h1>
        <span class="muted">组织身份、备案主域和归属审核的统一入口</span>
      </div>
      <a-space>
        <a-button @click="importOpen = true">导入 CSV</a-button>
        <a-button type="primary" @click="createOpen = true">新建组织</a-button>
      </a-space>
    </div>

    <a-alert v-if="importResult" type="info" show-icon closable class="import-result" @close="importResult = null">
      新建 {{ importResult.created }}，更新 {{ importResult.updated }}，候选域名 {{ importResult.domainsQueued }}
      <span v-if="importResult.errors?.length">，{{ importResult.errors.length }} 行失败</span>
    </a-alert>

    <a-card :bordered="false" class="org-card">
      <div class="toolbar">
        <a-input-search v-model:value="query" placeholder="搜索组织名称或别名" allow-clear @search="load" @change="onSearchChange" />
        <a-button @click="load">刷新</a-button>
      </div>
      <a-table
        class="org-table"
        :columns="columns"
        :data-source="rows"
        :loading="loading"
        row-key="id"
        :pagination="{ pageSize: 20, showSizeChanger: false }"
        @row-click="openOrganization"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'canonicalName'">
            <a class="org-name" @click.stop="openOrganization(record)">{{ record.canonicalName }}</a>
          </template>
          <template v-else-if="column.dataIndex === 'aliases'">{{ (record.aliases || []).join('、') || '-' }}</template>
          <template v-else-if="column.dataIndex === 'status'"><a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status }}</a-tag></template>
          <template v-else-if="column.dataIndex === 'actions'">
            <a-button size="small" @click.stop="openOrganization(record)">详情</a-button>
          </template>
          <template v-else>{{ record[column.dataIndex] ?? '-' }}</template>
        </template>
      </a-table>
    </a-card>

    <a-modal v-model:open="createOpen" title="新建组织" :confirm-loading="saving" @ok="createOrganization">
      <a-form layout="vertical">
        <a-form-item label="组织名称" required><a-input v-model:value="form.canonicalName" /></a-form-item>
        <a-form-item label="别名"><a-input v-model:value="form.aliases" placeholder="多个别名用 | 分隔" /></a-form-item>
        <a-form-item label="地区"><a-input v-model:value="form.region" placeholder="例如 蜀" /></a-form-item>
        <a-form-item label="统一社会信用代码"><a-input v-model:value="form.uscc" /></a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="importOpen" title="导入组织 CSV" :confirm-loading="importing" ok-text="上传" @ok="uploadCSV">
      <p class="muted">表头：canonicalName,aliases,region,uscc,rootDomains；别名用 |，域名用 ; 分隔。导入域名只生成候选，不会自动确认。</p>
      <input class="csv-input" type="file" accept=".csv,text/csv" @change="onFile" />
      <a-list v-if="importResult?.errors?.length" size="small" bordered class="row-errors">
        <a-list-item v-for="err in importResult.errors" :key="`${err.row}-${err.message}`">第 {{ err.row }} 行：{{ err.message }}</a-list-item>
      </a-list>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useRouter } from 'vue-router'
import { api, qs } from '@/api'
import type { Organization, OrganizationImportResult } from '@/components/easm/types'

const router = useRouter()
const rows = ref<Organization[]>([])
const loading = ref(false)
const query = ref('')
const createOpen = ref(false)
const importOpen = ref(false)
const saving = ref(false)
const importing = ref(false)
const importResult = ref<OrganizationImportResult | null>(null)
const selectedFile = ref<File | null>(null)
const form = reactive({ canonicalName: '', aliases: '', region: '', uscc: '' })
const columns = [
  { title: '组织', dataIndex: 'canonicalName', width: 280 },
  { title: '别名', dataIndex: 'aliases', width: 220 },
  { title: '地区', dataIndex: 'region', width: 100 },
  { title: '状态', dataIndex: 'status', width: 100 },
  { title: '操作', dataIndex: 'actions', width: 100 },
]

async function load() {
  loading.value = true
  try {
    const res = await api<{ data: { items: Organization[] } }>(`/organizations${qs({ query: query.value, limit: 200 })}`)
    rows.value = res.data.items || []
  } catch (e: any) {
    message.error(e.message || '加载组织失败')
  } finally {
    loading.value = false
  }
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
function onSearchChange() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(load, 250)
}

async function createOrganization() {
  if (!form.canonicalName.trim()) { message.warning('请输入组织名称'); return }
  saving.value = true
  try {
    await api('/organizations', {
      method: 'POST',
      body: JSON.stringify({
        canonicalName: form.canonicalName,
        aliases: form.aliases.split('|').map((x) => x.trim()).filter(Boolean),
        region: form.region,
        uscc: form.uscc,
      }),
    })
    message.success('组织已创建')
    createOpen.value = false
    Object.assign(form, { canonicalName: '', aliases: '', region: '', uscc: '' })
    await load()
  } catch (e: any) {
    message.error(e.message || '创建失败')
  } finally {
    saving.value = false
  }
}

function onFile(event: Event) {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0] || null
}

async function uploadCSV() {
  if (!selectedFile.value) { message.warning('请选择 CSV 文件'); return }
  importing.value = true
  try {
    const body = new FormData()
    body.append('file', selectedFile.value)
    const res = await api<{ data: OrganizationImportResult }>('/organizations/import', { method: 'POST', body })
    importResult.value = res.data
    message.success('导入完成')
    await load()
  } catch (e: any) {
    message.error(e.message || '导入失败')
  } finally {
    importing.value = false
  }
}

function openOrganization(record: Organization) {
  router.push(`/easm/organizations/${record.id}`)
}

onMounted(load)
</script>

<style scoped>
.easm-page-shell { display: flex; flex-direction: column; gap: 16px; }
.org-card { border-radius: 14px; }
.toolbar { display: flex; gap: 12px; margin-bottom: 16px; }
.toolbar :deep(.ant-input-search) { max-width: 420px; }
.org-table :deep(tr) { cursor: pointer; }
.org-name { font-weight: 600; }
.csv-input { display: block; margin: 16px 0; }
.import-result { border-radius: 10px; }
.row-errors { max-height: 220px; overflow: auto; }
</style>
