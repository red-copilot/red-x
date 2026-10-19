import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import AppLayout from '@/layouts/AppLayout.vue'
import LoginPage from '@/pages/LoginPage.vue'
import EasmPage from '@/pages/EasmPage.vue'
import TasksPage from '@/pages/TasksPage.vue'
import TaskDetailPage from '@/pages/TaskDetailPage.vue'
import AssetsPage from '@/pages/AssetsPage.vue'
import FindingsPage from '@/pages/FindingsPage.vue'
import ScopesPage from '@/pages/ScopesPage.vue'
import PoliciesPage from '@/pages/PoliciesPage.vue'
import MonitorsPage from '@/pages/MonitorsPage.vue'
import SchedulesPage from '@/pages/SchedulesPage.vue'
import GitHubPage from '@/pages/GitHubPage.vue'
import FingerprintsPage from '@/pages/FingerprintsPage.vue'
import PocPage from '@/pages/PocPage.vue'
import SystemPage from '@/pages/SystemPage.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: LoginPage, meta: { public: true } },
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', redirect: '/easm' },
        { path: 'easm', component: EasmPage, meta: { title: 'EASM 态势' } },
        { path: 'tasks', component: TasksPage, meta: { title: '任务' } },
        { path: 'tasks/:id', component: TaskDetailPage, meta: { title: '任务详情' } },
        { path: 'assets', component: AssetsPage, meta: { title: '资产' } },
        { path: 'findings', component: FindingsPage, meta: { title: '发现' } },
        { path: 'scopes', component: ScopesPage, meta: { title: '资产组' } },
        { path: 'policies', component: PoliciesPage, meta: { title: '策略' } },
        { path: 'monitors', component: MonitorsPage, meta: { title: '监控' } },
        { path: 'schedules', component: SchedulesPage, meta: { title: '计划任务' } },
        { path: 'github', component: GitHubPage, meta: { title: 'GitHub' } },
        { path: 'fingerprints', component: FingerprintsPage, meta: { title: '指纹库' } },
        { path: 'poc', component: PocPage, meta: { title: 'PoC 插件' } },
        { path: 'system', component: SystemPage, meta: { title: '系统' } },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true
  const auth = useAuthStore()
  await auth.fetchMe()
  if (!auth.user) return { path: '/login', query: { next: to.fullPath } }
  return true
})

window.addEventListener('arl:unauthorized', () => {
  if (router.currentRoute.value.path !== '/login') router.push('/login')
})

export default router
