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
    path: '/app',
    element: <AppLayout />,
    children: [
      { index: true, element: <Navigate to="/app/library" replace /> },
      { path: 'library', element: <LibraryPage /> },
      { path: 'chat', element: <ChatPage /> },
      { path: 'history', element: <HistoryPage /> },
    ],
  },
  { path: '/library', element: <Navigate to="/app/library" replace /> },
  { path: '/chat', element: <Navigate to="/app/chat" replace /> },
  { path: '/history', element: <Navigate to="/app/history" replace /> },
  {
    path: '*',
    element: <NotFoundPage />,
  },
])
