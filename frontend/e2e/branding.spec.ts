import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'

test('admin branding loads on sign-in and in both sidebar sizes', async ({ page }, testInfo) => {
  await page.goto('/login')
  const logo = page.getByRole('img', { name: 'PosternProxy', exact: true })
  await expect(logo).toBeVisible()
  await expect
    .poll(() => logo.evaluate((img: HTMLImageElement) => img.naturalWidth))
    .toBeGreaterThan(0)
  await page.screenshot({ path: testInfo.outputPath('sign-in.png') })
  await page.getByLabel('Email').fill('admin@e2e.test')
  await page.getByLabel('Password').fill('Disposable-browser-test-password')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
  await expect(logo).toBeVisible()
  await expect
    .poll(() => logo.evaluate((img: HTMLImageElement) => img.naturalWidth))
    .toBeGreaterThan(0)
  await page.screenshot({ path: testInfo.outputPath('sidebar.png') })
  await page.getByRole('button', { name: 'Collapse menu', exact: true }).click()
  await expect(logo).toBeVisible()
  await expect(page.getByRole('button', { name: 'Expand menu' })).toBeVisible()
  await page.getByRole('link', { name: 'PosternProxy dashboard' }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
  await page.getByRole('button', { name: 'Expand menu' }).click()
  await expect(page.getByRole('link', { name: 'Proxy Hosts', exact: true })).toBeVisible()
})

test('default pages follow system theme offline and fit mobile screens', async ({
  browser,
}, testInfo) => {
  const pages = JSON.parse(
    execFileSync('go', ['run', './e2e/testdata/pages'], {
      encoding: 'utf8',
      maxBuffer: 4 * 1024 * 1024,
    })
  ) as Record<string, string>
  const context = await browser.newContext({ javaScriptEnabled: false })
  const requests: string[] = []
  await context.route('**/*', (route) => {
    requests.push(route.request().url())
    return route.abort()
  })
  try {
    const page = await context.newPage()
    for (const [name, html] of Object.entries(pages)) {
      await page.setContent(html)
      const logo = page.getByRole('img', { name: 'PosternProxy', exact: true })
      for (const colorScheme of ['light', 'dark', 'light'] as const) {
        await page.emulateMedia({ colorScheme })
        await expect(logo).toBeVisible()
        await expect
          .poll(() => logo.evaluate((img: HTMLImageElement) => img.naturalWidth))
          .toBeGreaterThan(0)
        await expect
          .poll(() =>
            logo.evaluate(
              (img: HTMLImageElement) =>
                img.currentSrc === img.parentElement?.querySelector('source')?.srcset
            )
          )
          .toBe(colorScheme === 'dark')
        await expect(page.locator('main')).toHaveCSS(
          'background-color',
          colorScheme === 'dark' ? 'rgb(15, 23, 42)' : 'rgb(255, 255, 255)'
        )
        await expect(page.locator('body')).toHaveCSS(
          'color',
          colorScheme === 'dark' ? 'rgb(241, 245, 249)' : 'rgb(15, 23, 42)'
        )
        for (const width of [1280, 390, 320]) {
          await page.setViewportSize({ width, height: 900 })
          expect(
            await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)
          ).toBe(true)
        }
        if (name === 'proxy-502' || name === 'parked-410') {
          await page.setViewportSize({ width: 1280, height: 1000 })
          await page.screenshot({ path: testInfo.outputPath(`${name}-${colorScheme}.png`) })
        }
      }
    }
    expect(requests).toEqual([])
  } finally {
    await context.close()
  }
})
