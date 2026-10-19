<template>
  <div class="login-wrap">
    <div class="login-card">
      <div class="login-brand"><div class="mark">A</div><div><h1>ARL</h1><p>Asset Reconnaissance Lighthouse</p></div></div>
      <a-alert v-if="error" type="error" :message="error" show-icon style="margin-bottom: 16px" />
      <a-form layout="vertical" @finish="submit">
        <a-form-item label="用户名" name="username" :rules="[{ required: true, message: '请输入用户名' }]">
          <a-input v-model:value="username" size="large" autocomplete="username" />
        </a-form-item>
        <a-form-item label="密码" name="password" :rules="[{ required: true, message: '请输入密码' }]">
          <a-input-password v-model:value="password" size="large" autocomplete="current-password" />
        </a-form-item>
        <a-button type="primary" html-type="submit" size="large" block :loading="loading">登录</a-button>
      </a-form>
      <p class="muted login-foot">首次启动时管理员密码会在服务日志中输出一次。</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const username = ref('admin')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    router.push((route.query.next as string) || '/')
  } catch (e: any) {
    error.value = e.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap { min-height: 100vh; display: grid; place-items: center; background: radial-gradient(circle at 20% 10%, #1d3b74 0, transparent 35%), radial-gradient(circle at 90% 90%, #3b1d66 0, transparent 40%), #080e1b; padding: 24px; }
.login-card { width: min(420px, 100%); background: #fff; border-radius: 20px; padding: 32px; box-shadow: 0 30px 80px rgba(0,0,0,.35); }
.login-brand { display: flex; gap: 14px; align-items: center; margin-bottom: 26px; }
.login-brand h1 { margin: 0; font-size: 28px; }
.login-brand p { margin: 2px 0 0; color: #6b7280; font-size: 12px; }
.mark { width: 52px; height: 52px; border-radius: 15px; display: grid; place-items: center; color: #fff; font-weight: 800; font-size: 24px; background: linear-gradient(135deg,#4f8cff,#8b5cf6); }
.login-foot { margin: 18px 0 0; font-size: 12px; text-align: center; }
</style>
