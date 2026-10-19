<template>
  <a-layout style="min-height: 100vh">
    <a-layout-sider v-model:collapsed="collapsed" collapsible :width="236" theme="dark">
      <div class="brand">
        <div class="brand-mark">A</div>
        <div v-if="!collapsed">
          <strong>ARL</strong>
          <div class="brand-sub">Asset Reconnaissance</div>
        </div>
      </div>
      <a-menu v-model:selectedKeys="selected" theme="dark" mode="inline" :items="menuItems" @click="onMenu" />
    </a-layout-sider>
    <a-layout>
      <a-layout-header class="header">
        <div>
          <div class="header-title">{{ route.meta.title || '控制台' }}</div>
          <div class="muted header-sub">模块化单体 · 原生扫描引擎</div>
        </div>
        <a-space>
          <a-tag color="green">{{ auth.user?.role || 'viewer' }}</a-tag>
          <a-dropdown>
            <a-button type="text">{{ auth.user?.username || '未登录' }} <DownOutlined /></a-button>
            <template #overlay>
              <a-menu>
                <a-menu-item key="logout" @click="logout">退出登录</a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </a-space>
      </a-layout-header>
      <a-layout-content class="content">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { DownOutlined } from '@ant-design/icons-vue'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const collapsed = ref(false)
const selected = computed(() => [route.path.split('/')[1] || 'easm'])

const menuItems = [
  { key: 'easm', label: 'EASM 态势' },
  { key: 'tasks', label: '任务' },
  { key: 'assets', label: '资产' },
  { key: 'findings', label: '发现' },
  { key: 'scopes', label: '资产组' },
  { key: 'policies', label: '策略' },
  { key: 'monitors', label: '监控' },
  { key: 'schedules', label: '计划任务' },
  { key: 'github', label: 'GitHub' },
  { key: 'fingerprints', label: '指纹库' },
  { key: 'poc', label: 'PoC 插件' },
  { key: 'system', label: '系统' },
]

function onMenu({ key }: { key: string }) {
  router.push(`/${key}`)
}

async function logout() {
  await auth.logout()
  router.push('/login')
}
</script>

<style scoped>
.brand { height: 68px; display: flex; align-items: center; gap: 12px; padding: 0 18px; color: #fff; }
.brand-mark { width: 34px; height: 34px; border-radius: 9px; background: linear-gradient(135deg,#4f8cff,#8b5cf6); display:grid; place-items:center; font-weight:800; }
.brand-sub { font-size: 10px; color: #8b98ad; letter-spacing: .08em; text-transform: uppercase; }
.header { background: #fff; padding: 0 26px; display:flex; align-items:center; justify-content:space-between; border-bottom:1px solid #e5eaf2; height:68px; }
.header-title { font-weight: 650; }
.header-sub { font-size: 12px; }
.content { padding: 24px; }
</style>
