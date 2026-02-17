import { createBrowserRouter } from 'react-router-dom'

import { ChatPage } from '@/pages/ChatPage'
import { HomePage } from '@/pages/HomePage'
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
    path: '*',
    element: <NotFoundPage />,
  },
])
