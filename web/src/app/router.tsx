import { lazy, Suspense, type ReactNode } from 'react'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { AdminRoute, ProtectedRoute } from './auth'
import { AppShell } from '@/components/layout/app-shell'
import { LoadingBlock } from '@/components/common/states'

const LoginPage = lazy(() => import('@/pages/login-page').then((module) => ({ default: module.LoginPage })))
const ChangePasswordPage = lazy(() =>
  import('@/pages/change-password-page').then((module) => ({ default: module.ChangePasswordPage })),
)
const OverviewPage = lazy(() => import('@/pages/overview-page').then((module) => ({ default: module.OverviewPage })))
const OrganizationsPage = lazy(() =>
  import('@/pages/organizations-page').then((module) => ({ default: module.OrganizationsPage })),
)
const OrganizationDetailPage = lazy(() =>
  import('@/pages/organization-detail-page').then((module) => ({ default: module.OrganizationDetailPage })),
)
const AssetsPage = lazy(() => import('@/pages/assets-page').then((module) => ({ default: module.AssetsPage })))
const FindingsPage = lazy(() => import('@/pages/findings-page').then((module) => ({ default: module.FindingsPage })))
const TasksPage = lazy(() => import('@/pages/tasks-page').then((module) => ({ default: module.TasksPage })))
const TaskDetailPage = lazy(() =>
  import('@/pages/task-detail-page').then((module) => ({ default: module.TaskDetailPage })),
)
const MonitorsPage = lazy(() => import('@/pages/monitors-page').then((module) => ({ default: module.MonitorsPage })))
const SchedulesPage = lazy(() => import('@/pages/schedules-page').then((module) => ({ default: module.SchedulesPage })))
const PoliciesPage = lazy(() => import('@/pages/policies-page').then((module) => ({ default: module.PoliciesPage })))
const ScopesPage = lazy(() => import('@/pages/scopes-page').then((module) => ({ default: module.ScopesPage })))
const GitHubPage = lazy(() => import('@/pages/github-page').then((module) => ({ default: module.GitHubPage })))
const FingerprintsPage = lazy(() =>
  import('@/pages/fingerprints-page').then((module) => ({ default: module.FingerprintsPage })),
)
const PocPage = lazy(() => import('@/pages/poc-page').then((module) => ({ default: module.PocPage })))
const UsersPage = lazy(() => import('@/pages/users-page').then((module) => ({ default: module.UsersPage })))
const APIKeysPage = lazy(() => import('@/pages/api-keys-page').then((module) => ({ default: module.APIKeysPage })))
const AuditPage = lazy(() => import('@/pages/audit-page').then((module) => ({ default: module.AuditPage })))
const SystemPage = lazy(() => import('@/pages/system-page').then((module) => ({ default: module.SystemPage })))
const NotFoundPage = lazy(() => import('@/pages/not-found-page').then((module) => ({ default: module.NotFoundPage })))

function page(element: ReactNode) {
  return <Suspense fallback={<LoadingBlock />}>{element}</Suspense>
}

const router = createBrowserRouter([
  { path: '/login', element: page(<LoginPage />) },
  { path: '/change-password', element: <ProtectedRoute>{page(<ChangePasswordPage />)}</ProtectedRoute> },
  {
    path: '/',
    element: (
      <ProtectedRoute>
        <AppShell />
      </ProtectedRoute>
    ),
    children: [
      { index: true, element: page(<OverviewPage />) },
      { path: 'organizations', element: page(<OrganizationsPage />) },
      { path: 'organizations/:id', element: page(<OrganizationDetailPage />) },
      { path: 'assets', element: page(<AssetsPage />) },
      { path: 'findings', element: page(<FindingsPage />) },
      { path: 'tasks', element: page(<TasksPage />) },
      { path: 'tasks/:id', element: page(<TaskDetailPage />) },
      { path: 'automation/monitors', element: page(<MonitorsPage />) },
      { path: 'automation/schedules', element: page(<SchedulesPage />) },
      { path: 'automation/policies', element: page(<PoliciesPage />) },
      { path: 'automation/groups', element: page(<ScopesPage />) },
      { path: 'intelligence/github', element: page(<GitHubPage />) },
      { path: 'intelligence/fingerprints', element: page(<FingerprintsPage />) },
      { path: 'intelligence/poc', element: page(<PocPage />) },
      { path: 'admin/users', element: <AdminRoute>{page(<UsersPage />)}</AdminRoute> },
      { path: 'admin/api-keys', element: <AdminRoute>{page(<APIKeysPage />)}</AdminRoute> },
      { path: 'admin/audit', element: <AdminRoute>{page(<AuditPage />)}</AdminRoute> },
      { path: 'admin/system', element: <AdminRoute>{page(<SystemPage />)}</AdminRoute> },
      { path: '*', element: page(<NotFoundPage />) },
    ],
  },
])

export function AppRouter() {
  return <RouterProvider router={router} />
}
