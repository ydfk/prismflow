import { expect, test } from '@playwright/test'

test('总览按时段与协议显示历史用量及活动连接', async ({ page }) => {
  const time = Math.floor(Date.now() / 300000) * 300
  await page.route('**/api/state', route => route.fulfill({ json: {
    state: { endpoints: [], users: [], certificates: [] },
    traffic: { used: 0, available: true, interface: 'eth0' },
    core: { available: true, running: true },
    metrics: {
      online: true, sampledAt: new Date().toISOString(),
      activeConnections: { total: 4, vless: 3, hysteria2: 1 },
      rates: [{ protocol: 'total', upload: 12, download: 36 }, { protocol: 'vless', upload: 8, download: 24 }],
    },
  } }))
  await page.route('**/api/metrics?*', route => {
    const query = new URL(route.request().url()).searchParams
    expect(query.get('bucket')).toBe('auto')
    return route.fulfill({ json: query.get('range') === '7d' ? [] : [
      { time: time - 300, protocol: 'total', upload: 1024, download: 4096 },
      { time: time - 300, protocol: 'vless', upload: 1024, download: 2048 },
      { time, protocol: 'total', upload: 1024, download: 2048 },
      { time, protocol: 'hysteria2', upload: 1024, download: 0 },
    ] })
  })
  await page.route('**/api/status', route => route.fulfill({ json: { setupRequired: false } }))
  await page.goto('/')

  const vless = page.getByRole('row', { name: /VLESS/ })
  await expect(vless.getByRole('cell')).toHaveText(['1.0 KiB', '2.0 KiB', '3.0 KiB', '75%', '3'])
  await vless.getByRole('button', { name: 'VLESS' }).click()
  await expect(page.getByLabel('流量协议')).toHaveValue('vless')
  await expect(page.locator('.traffic-readings')).toContainText('3.0 KiB')
  await expect(page.getByRole('img', { name: /每 5 分钟上传与下载趋势/ })).toBeVisible()

  await page.setViewportSize({ width: 390, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.getByLabel('流量时间范围').selectOption('7d')
  await expect(page.getByText('所选时段没有流量记录')).toBeVisible()
  await expect(vless.getByRole('cell')).toHaveText(['—', '—', '—', '—', '3'])
})
