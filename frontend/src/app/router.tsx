import { Navigate, createBrowserRouter } from 'react-router-dom'

import { AdminAwayGuard } from '@/pages/admin/AdminAwayGuard'
import { AdminRouteGuard } from '@/pages/admin/AdminRouteGuard'
import { AdminLayout } from '@/pages/admin/AdminLayout'

export const router = createBrowserRouter([
  {
    path: '/',
    lazy: async () => {
      const module = await import('@/pages/HomePage')
      return { Component: module.HomePage }
    },
  },
  {
    element: <AdminAwayGuard />,
    children: [
      {
        path: '/log-in',
        lazy: async () => {
          const module = await import('@/pages/LoginPage')
          return { Component: module.LoginPage }
        },
      },
      {
        path: '/create-account',
        lazy: async () => {
          const module = await import('@/pages/LoginPage')
          return { Component: module.LoginPage }
        },
      },
      {
        path: '/log-in/password',
        lazy: async () => {
          const module = await import('@/pages/LoginPage')
          return { Component: module.LoginPage }
        },
      },
      {
        path: '/create-account/password',
        lazy: async () => {
          const module = await import('@/pages/LoginPage')
          return { Component: module.LoginPage }
        },
      },
      {
        path: '/log-in/verify',
        lazy: async () => {
          const module = await import('@/pages/LoginPage')
          return { Component: module.LoginPage }
        },
      },
      {
        path: '/log-in/error',
        lazy: async () => {
          const module = await import('@/pages/LoginPage')
          return { Component: module.LoginPage }
        },
      },
      {
        path: '/email-verification',
        lazy: async () => {
          const module = await import('@/pages/LoginPage')
          return { Component: module.LoginPage }
        },
      },
      {
        path: '/reset-password',
        lazy: async () => {
          const module = await import('@/pages/LoginPage')
          return { Component: module.LoginPage }
        },
      },
      {
        path: '/reset-password/new-password',
        lazy: async () => {
          const module = await import('@/pages/LoginPage')
          return { Component: module.LoginPage }
        },
      },
      {
        path: '/reset-password/success',
        lazy: async () => {
          const module = await import('@/pages/LoginPage')
          return { Component: module.LoginPage }
        },
      },
      {
        path: '/chat',
        lazy: async () => {
          const module = await import('@/pages/ChatPage')
          return { Component: module.ChatPage }
        },
      },
      {
        path: '/chat/:conversationId',
        lazy: async () => {
          const module = await import('@/pages/ChatPage')
          return { Component: module.ChatPage }
        },
      },
      {
        path: '/library',
        lazy: async () => {
          const module = await import('@/pages/LibraryPage')
          return { Component: module.LibraryPage }
        },
      },
    ],
  },
  {
    path: '/admin',
    element: <AdminRouteGuard />,
    children: [
      {
        element: <AdminLayout />,
        children: [
          { index: true, element: <Navigate to="/admin/overview" replace /> },
          {
            path: 'overview',
            lazy: async () => {
              const module = await import('@/pages/admin/AdminOverviewPage')
              return { Component: module.AdminOverviewPage }
            },
          },
          {
            path: 'users',
            lazy: async () => {
              const module = await import('@/pages/admin/AdminUsersPage')
              return { Component: module.AdminUsersPage }
            },
          },
          {
            path: 'books',
            lazy: async () => {
              const module = await import('@/pages/admin/AdminBooksPage')
              return { Component: module.AdminBooksPage }
            },
          },
          {
            path: 'evals',
            lazy: async () => {
              const module = await import('@/pages/admin/AdminEvalsPage')
              return { Component: module.AdminEvalsPage }
            },
          },
          {
            path: 'audit',
            lazy: async () => {
              const module = await import('@/pages/admin/AdminAuditPage')
              return { Component: module.AdminAuditPage }
            },
          },
        ],
      },
    ],
  },
  {
    path: '*',
    lazy: async () => {
      const module = await import('@/pages/NotFoundPage')
      return { Component: module.NotFoundPage }
    },
  },
])
