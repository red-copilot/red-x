<template>
  <div>
    <div class="page-title"><div><h1>资产态势</h1><span class="muted">任务、资产与运行状态的实时概览</span></div><a-button @click="load">刷新</a-button></div>
    <div class="metric-grid">
      <div v-for="item in taskMetrics" :key="item.label" class="metric-card">
        <div class="label">{{ item.label }}</div><div class="value">{{ item.value }}</div>
      </div>
    </div>
    <a-row :gutter="[16,16]" style="margin-top:18px">
      <a-col :xs="24" :lg="12">
        <a-card title="资产分布" :bordered="false" style="border-radius:14px">
          <div class="metric-grid">
            <div v-for="(value, key) in assetCounts" :key="key" class="metric-card" style="padding:14px">
              <div class="label">{{ key }}</div><div class="value" style="font-size:22px">{{ value }}</div>
            </div>
          </div>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="12">
        <a-card title="系统状态" :bordered="false" style="border-radius:14px">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="版本">{{ info.version || '-' }}</a-descriptions-item>
            <a-descriptions-item label="运行时长">{{ info.uptimeSec || 0 }} 秒</a-descriptions-item>
            <a-descriptions-item label="队列">{{ info.queueDriver || '-' }}</a-descriptions-item>
            <a-descriptions-item label="健康检查"><a-tag :color="health === 'ok' ? 'green' : 'red'">{{ health }}</a-tag></a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
    </a-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '@/api'

const taskCounts = ref<Record<string, number>>({})
const assetCounts = ref<Record<string, number>>({})
const info = ref<Record<string, any>>({})
const health = ref('checking')

const taskMetrics = computed(() => [
  { label: '运行中', value: taskCounts.value.running || 0 },
  { label: '排队中', value: taskCounts.value.queued || 0 },
  { label: '已完成', value: taskCounts.value.succeeded || 0 },
  { label: '部分完成', value: taskCounts.value.partial || 0 },
  { label: '失败', value: taskCounts.value.failed || 0 },
])

async function load() {
  const [dash, sys, ready] = await Promise.all([
    api<{ data: { tasks: Record<string, number>; assets: Record<string, number> } }>('/dashboard'),
    api<{ data: Record<string, any> }>('/system/info'),
    fetch('/readyz').then((r) => r.ok).catch(() => false),
  ])
  taskCounts.value = dash.data.tasks || {}
  assetCounts.value = dash.data.assets || {}
  info.value = sys.data || {}
  health.value = ready ? 'ok' : 'degraded'
}

onMounted(load)
</script>
