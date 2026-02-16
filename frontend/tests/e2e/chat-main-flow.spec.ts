import { expect, test } from '@playwright/test'

test('authenticated user can ask a question and receive cited answer', async ({ page }) => {
  await page.route('**/api/users/me', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: 'user-e2e',
        email: 'e2e@example.com',
        role: 'user',
        status: 'active',
      }),
    })
  })

  await page.route('**/api/books', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback()
      return
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: [
          {
            id: 'book-ready',
            title: '测试书籍',
            status: 'ready',
          },
        ],
        count: 1,
      }),
    })
  })

  await page.route('**/api/chats', async (route) => {
    const payload = route.request().postDataJSON() as { bookId: string; question: string }
    expect(payload.bookId).toBe('book-ready')
    expect(payload.question).toContain('第一章')
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        bookId: 'book-ready',
        question: payload.question,
        answer: '这是基于书籍内容的回答，重点在第一章。',
        createdAt: new Date().toISOString(),
        sources: [
          {
            label: '[1]',
            location: 'page 1',
            snippet: '第一章核心观点摘要',
          },
        ],
      }),
    })
  })

  await page.goto('/chat')

  const composer = page.getByLabel('输入你的问题').first()
  await composer.click()
  await composer.fill('请总结第一章的核心观点。')

  const sendButton = page.getByTestId('send-button').first()
  await expect(sendButton).toBeEnabled()
  await sendButton.click()

  await expect(page.getByText('这是基于书籍内容的回答，重点在第一章。')).toBeVisible()
  await expect(page.getByText('page 1')).toBeVisible()
})
