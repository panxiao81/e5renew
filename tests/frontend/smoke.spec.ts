import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('home page renders localized content', async ({ page }) => {
  await page.goto('/')

  await expect(page).toHaveTitle(/E5Renew/)
  await expect(page.getByRole('heading', { name: 'Welcome to E5 Application!' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Login' })).toBeVisible()
})

test('language query switches UI to Chinese', async ({ page }) => {
  await page.goto('/?lang=zh')

  await expect(page.getByRole('heading', { name: '欢迎使用 E5 应用程序！' })).toBeVisible()
  await expect(page.getByRole('link', { name: '首页' })).toBeVisible()
  await expect(page.getByRole('link', { name: '登录' })).toBeVisible()
})

test('language choice persists through cookie on next page load', async ({ page }) => {
  await page.goto('/?lang=zh')
  await page.goto('/')

  await expect(page.getByRole('heading', { name: '欢迎使用 E5 应用程序！' })).toBeVisible()
  await expect(page.getByRole('link', { name: '首页' })).toBeVisible()
})

test('about page is reachable', async ({ page }) => {
  await page.goto('/about')

  await expect(page.getByText('About E5Renew: A tool for renewing Microsoft E5 licenses.')).toBeVisible()
})

test('user page redirects unauthenticated visitors to login', async ({ page }) => {
  await page.goto('/user')

  await expect(page).toHaveURL(/\/login$/)
  await expect(page.getByText('Login placeholder')).toBeVisible()
})

test('test login route renders authenticated user page', async ({ page }) => {
  await page.goto('/test/login')

  await expect(page).toHaveURL(/\/user$/)
  await expect(page.getByText('Frontend Test User')).toBeVisible()
  await expect(page.getByText('❌ Not Authorized')).toBeVisible()
  await expect(page.getByRole('link', { name: 'API Logs' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Logout' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Login' })).toHaveCount(0)
})

test('logs page shows seeded stats and filter state for authenticated user', async ({ page }) => {
  await page.goto('/test/login')
  await page.goto('/logs?job_type=client_credentials&time_range=7d')

  await expect(page.getByRole('heading', { name: /API Logs/ })).toBeVisible()
  await expect(page.getByRole('link', { name: 'API Logs' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Logout' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Login' })).toHaveCount(0)
  await expect(page.getByRole('heading', { name: 'Total Requests' })).toBeVisible()
  await expect(page.locator('strong').filter({ hasText: '3' }).first()).toBeVisible()
  await expect(page.locator('code').filter({ hasText: 'users' }).first()).toBeVisible()
  await expect(page.getByText('graph request failed')).toBeVisible()
  await expect(page.locator('#job_type')).toHaveValue('client_credentials')
  await expect(page.locator('#time_range')).toHaveValue('7d')
})

test('logs stats page shows detailed statistics view', async ({ page }) => {
  await page.goto('/test/login')
  await page.goto('/logs/stats?time_range=24h')

  await expect(page.getByRole('heading', { name: /API Statistics/ })).toBeVisible()
  await expect(page.getByText('me/messages')).toBeVisible()
  await expect(page.getByText('66.7%')).toBeVisible()
})

test('static asset endpoint serves bundled javascript', async ({ request }) => {
  const response = await request.get('/statics/main.js')

  expect(response.ok()).toBeTruthy()
  expect(await response.text()).toContain('convertTimestamps')
})

test('home page remains usable on mobile viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')

  await expect(page.getByRole('heading', { name: 'Welcome to E5 Application!' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Home' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Login' })).toBeVisible()
})

test('authenticated logs page remains usable on mobile viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/test/login')
  await page.goto('/logs?job_type=client_credentials&time_range=7d')

  await expect(page.getByRole('heading', { name: /API Logs/ })).toBeVisible()
  await expect(page.locator('#job_type')).toHaveValue('client_credentials')
  await expect(page.locator('#time_range')).toHaveValue('7d')
})

test('core pages pass accessibility smoke checks', async ({ page }) => {
  const urls = ['/', '/test/login', '/logs', '/logs/stats']

  for (const url of urls) {
    await page.goto(url)

    const accessibilityScanResults = await new AxeBuilder({ page })
      .disableRules(['color-contrast'])
      .analyze()

    expect(accessibilityScanResults.violations, `${url} accessibility violations`).toEqual([])
  }
})
