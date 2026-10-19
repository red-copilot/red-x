<template>
  <a-card :bordered="false" style="border-radius: 14px">
    <div class="table-toolbar">
      <a-input-search v-model:value="search" placeholder="搜索" style="max-width: 320px" @search="reload" />
      <slot name="toolbar" />
      <a-button @click="reload">刷新</a-button>
    </div>
    <a-table
      :columns="columns"
      :data-source="rows"
      :loading="loading"
      :pagination="{ current: page, pageSize, total, showSizeChanger: true }"
      :scroll="{ x: 900 }"
      row-key="id"
      size="middle"
      @change="onTableChange"
    >
      <template #bodyCell="{ column, record }">
        <slot name="cell" :column="column" :record="record">
          <span v-if="column.dataIndex === 'createdAt'">{{ formatTime(record.createdAt) }}</span>
          <span v-else-if="column.dataIndex === 'status'"><a-tag>{{ record.status }}</a-tag></span>
          <span v-else>{{ display(record, column.dataIndex) }}</span>
        </slot>
      </template>
    </a-table>
  </a-card>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { api, formatTime, qs } from '@/api'

const props = defineProps<{
  endpoint: string
  columns: { title: string; dataIndex: string; key?: string; width?: number }[]
  params?: Record<string, unknown>
}>()

const rows = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const search = ref('')
const loading = ref(false)

function display(record: any, key: string) {
  const value = record?.[key]
  if (value === undefined || value === null || value === '') return '-'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

async function load() {
  loading.value = true
  try {
    const res = await api<{ data: { items: any[]; total: number } }>(
      `${props.endpoint}${qs({ page: page.value, pageSize: pageSize.value, q: search.value, ...(props.params || {}) })}`,
    )
    rows.value = res.data.items || []
    total.value = res.data.total || rows.value.length
  } finally {
    loading.value = false
  }
}

function reload() {
  page.value = 1
  load()
}

function onTableChange(pagination: any) {
  page.value = pagination.current
  pageSize.value = pagination.pageSize
  load()
}

watch(() => props.params, reload, { deep: true })
onMounted(load)
defineExpose({ reload })
</script>
