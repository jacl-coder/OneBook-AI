import { Navigate, createBrowserRouter } from 'react-router-dom'

import { ChatPage } from '@/pages/ChatPage'
import { HomePage } from '@/pages/HomePage'
import { LibraryPage } from '@/pages/LibraryPage'
import { LoginPage } from '@/pages/LoginPage'
import { NotFoundPage } from '@/pages/NotFoundPage'
import { AdminRouteGuard } from '@/pages/admin/AdminRouteGuard'
import { AdminLayout } from '@/pages/admin/AdminLayout'
import { AdminOverviewPage } from '@/pages/admin/AdminOverviewPage'
import { AdminUsersPage } from '@/pages/admin/AdminUsersPage'
import { AdminBooksPage } from '@/pages/admin/AdminBooksPage'
import { AdminAuditPage } from '@/pages/admin/AdminAuditPage'
import { AdminEvalsPage } from '@/pages/admin/AdminEvalsPage'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <HomePage />,
  },
  {
    path: '/log-in',
    element: <LoginPage />,
  },
  {
    path: '/create-account',
    element: <LoginPage />,
  },
  {
    path: '/log-in/password',
    element: <LoginPage />,
  },
  {
    path: '/create-account/password',
    element: <LoginPage />,
  },
  {
    path: '/log-in/verify',
    element: <LoginPage />,
  },
  {
    path: '/log-in/error',
    element: <LoginPage />,
  },
  {
    path: '/email-verification',
    element: <LoginPage />,
  },
  {
    path: '/reset-password',
    element: <LoginPage />,
  },
  {
    path: '/reset-password/new-password',
    element: <LoginPage />,
  },
  {
    path: '/reset-password/success',
    element: <LoginPage />,
  },
  {
    path: '/chat',
    element: <ChatPage />,
  },
  {
    path: '/chat/:conversationId',
    element: <ChatPage />,
  },
  {
    path: '/library',
    element: <LibraryPage />,
  },
  {
    path: '/admin',
    element: <AdminRouteGuard />,
    children: [
      {
        element: <AdminLayout />,
        children: [
          { index: true, element: <Navigate to="/admin/overview" replace /> },
          { path: 'overview', element: <AdminOverviewPage /> },
          { path: 'users', element: <AdminUsersPage /> },
          { path: 'books', element: <AdminBooksPage /> },
          { path: 'evals', element: <AdminEvalsPage /> },
          { path: 'audit', element: <AdminAuditPage /> },
        ],
      },
    ],
  },
  {
    path: '*',
    element: <NotFoundPage />,
  },
])
