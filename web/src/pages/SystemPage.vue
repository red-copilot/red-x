<template>
  <div>
    <div class="page-title"><div><h1>系统</h1><span class="muted">运行状态、API 文档与可观测性入口</span></div><a-button @click="load">刷新</a-button></div>
    <a-row :gutter="[16,16]">
      <a-col :xs="24" :lg="12">
        <a-card title="运行信息" :bordered="false" style="border-radius:14px">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="版本">{{ info.version }}</a-descriptions-item>
            <a-descriptions-item label="运行时长">{{ info.uptimeSec }} 秒</a-descriptions-item>
            <a-descriptions-item label="队列驱动">{{ info.queueDriver }}</a-descriptions-item>
            <a-descriptions-item label="健康"><a-tag :color="health ? 'green' : 'red'">{{ health ? 'ready' : 'degraded' }}</a-tag></a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="12">
        <a-card title="接口与监控" :bordered="false" style="border-radius:14px">
          <a-space direction="vertical">
            <a href="/api/v1/docs" target="_blank">API 文档</a>
            <a href="/api/v1/openapi.json" target="_blank">OpenAPI 3.1</a>
            <a href="/metrics" target="_blank">Prometheus metrics</a>
            <a href="/readyz" target="_blank">Readyz</a>
          </a-space>
        </a-card>
      </a-col>
    </a-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '@/api'

const info = ref<Record<string, any>>({})
const health = ref(false)
async function load() {
  const res = await api<{ data: Record<string, any> }>('/system/info')
  info.value = res.data
  health.value = (await fetch('/readyz')).ok
}
onMounted(load)
</script>
