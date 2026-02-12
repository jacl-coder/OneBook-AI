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
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/create-account',
    element: <LoginPage />,
  },
  {
    path: '/login/password',
    element: <LoginPage />,
  },
  {
    path: '/login/verify',
    element: <LoginPage />,
  },
  {
    path: '/email-verification',
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
  { path: '/app', element: <Navigate to="/library" replace /> },
  { path: '/app/library', element: <Navigate to="/library" replace /> },
  { path: '/app/chat', element: <Navigate to="/chat" replace /> },
  { path: '/app/history', element: <Navigate to="/history" replace /> },
  {
    path: '*',
    element: <NotFoundPage />,
  },
])
