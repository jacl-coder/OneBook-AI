import { Navigate, createBrowserRouter } from 'react-router-dom'

import { AppLayout } from '@/app/ui/AppLayout'
import { ChatPage } from '@/pages/ChatPage'
import { HistoryPage } from '@/pages/HistoryPage'
import { HomePage } from '@/pages/HomePage'
import { LibraryPage } from '@/pages/LibraryPage'
import { LoginPage } from '@/pages/LoginPage'
import { NotFoundPage } from '@/pages/NotFoundPage'

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
    path: '/library',
    element: <AppLayout />,
    children: [
      { index: true, element: <LibraryPage /> },
    ],
  },
  {
    path: '/chat',
    element: <AppLayout />,
    children: [{ index: true, element: <ChatPage /> }],
  },
  {
    path: '/history',
    element: <AppLayout />,
    children: [{ index: true, element: <HistoryPage /> }],
  },
  {
    path: '*',
    element: <NotFoundPage />,
  },
])
