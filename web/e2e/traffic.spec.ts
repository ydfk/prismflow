import { expect, test } from '@playwright/test'

test.use({ timezoneId: 'Asia/Shanghai' })

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

test('按 IP 查询分钟流量，从时间明细反查用量并导出', async ({ page }) => {
  await page.route('**/api/status', route => route.fulfill({ json: { setupRequired: false } }))
  await page.route('**/api/state', route => route.fulfill({ json: {
    state: { endpoints: [], users: [], certificates: [], retentionDays: 0 },
    traffic: { used: 0, available: true, interface: 'eth0' },
    core: { available: true, running: true },
    metrics: { online: true, sampledAt: new Date().toISOString(), rates: [] },
  } }))
  await page.route('**/api/metrics?*', route => route.fulfill({ json: [] }))
  const minute = Math.floor(Date.now() / 60000) * 60
  await page.route('**/api/metrics/history?*', route => {
    const query = new URL(route.request().url()).searchParams
    const step = query.get('granularity') === 'minute' ? 60 : 3600
    const upload = query.get('ip') === '192.0.2.1' ? 1024 : 8192
    return route.fulfill({ json: { from: minute - 3600, until: minute + 3600,
      buckets: [{ time: minute, end: minute + step, upload, download: 4096 }],
    } })
  })
  await page.route('**/api/metrics/breakdown?*', route => {
    const query = new URL(route.request().url()).searchParams
    const value = query.get('group') === 'network' ? 'tcp' : query.get('page') === '2' ? '2001:db8::1' : '192.0.2.1'
    return route.fulfill({ json: { rows: [{ value, upload: 1024, download: 4096 }], hasMore: !query.has('ip') && query.get('page') === '1' } })
  })
  await page.goto('/')
  await page.getByRole('button', { name: '查看历史' }).click()
  await expect(page.getByRole('heading', { name: '流量统计', exact: true })).toBeVisible()
  await expect(page.getByText(/不按天清理/)).toBeVisible()
  await page.getByRole('button', { name: '下一页' }).click()
  await expect(page.getByRole('button', { name: '2001:db8::1' })).toBeVisible()
  await page.getByRole('button', { name: '上一页' }).click()
  await page.getByRole('button', { name: '192.0.2.1' }).click()
  await expect(page.getByLabel('来源 IP', { exact: true })).toHaveValue('192.0.2.1')
  await page.getByLabel('统计粒度').selectOption('minute')
  await page.getByRole('button', { name: '查询', exact: true }).click()
  await expect(page.getByRole('img', { name: /每 1 分钟上传与下载趋势/ })).toBeVisible()
  await page.getByRole('button', { name: '选择图表时段' }).click()
  await expect(page.getByRole('region', { name: '所选时段流量明细' })).toContainText('5.0 KiB')
  await page.getByRole('button', { name: '清除时段选择' }).click()
  const sliceRequest = page.waitForRequest(request => {
    const url = new URL(request.url())
    return url.pathname.endsWith('/metrics/breakdown') && url.searchParams.get('slot') === String(minute) &&
      url.searchParams.get('ip') === '192.0.2.1'
  })
  await page.getByRole('region', { name: '时间用量明细' }).getByRole('button').click()
  await sliceRequest
  await page.getByLabel('用量分组').selectOption('network')
  await expect(page.getByRole('button', { name: 'TCP' })).toBeVisible()
  await page.getByRole('button', { name: '查看完整时段' }).click()
  const download = page.waitForEvent('download')
  await page.getByRole('button', { name: '导出 CSV' }).click()
  expect((await download).suggestedFilename()).toMatch(/^prismflow-traffic-.*\.csv$/)
  await page.getByRole('button', { name: '移除来源 IP筛选' }).click()
  await expect(page.getByLabel('来源 IP', { exact: true })).toHaveValue('')

  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('button', { name: '切换浅色主题' }).click()
  await expect(page.getByRole('heading', { name: '流量趋势' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})

test('按天趋势标出每一天，点选后联动用量排行和当天小时明细', async ({ page }) => {
  await page.clock.setFixedTime(new Date('2026-11-22T04:00:00Z'))
  await page.route('**/api/status', route => route.fulfill({ json: { setupRequired: false } }))
  await page.route('**/api/state', route => route.fulfill({ json: {
    state: { endpoints: [], users: [], certificates: [], retentionDays: 30 },
    traffic: { used: 0 },
    core: { available: true, running: true },
    metrics: { online: true, sampledAt: new Date().toISOString(), rates: [] },
  } }))
  await page.route('**/api/metrics?*', route => route.fulfill({ json: [] }))
  const day = 86400
  const first = Date.parse('2026-10-24T00:00:00+08:00') / 1000
  const starts = Array.from({ length: 30 }, (_, index) => first + index * day)
  const peak = starts[12]
  const latest = starts.at(-1)!
  await page.route('**/api/metrics/history?*', route => {
    const query = new URL(route.request().url()).searchParams
    if (query.get('granularity') === 'day') {
      return route.fulfill({ json: { from: first, until: first + 30 * day, starts,
        buckets: starts.slice(7).map(time => ({ time, end: time + day, upload: 1024, download: time === peak ? 8192 : 2048 })),
      } })
    }
    return route.fulfill({ json: { from: latest, until: latest + day, buckets: [] } })
  })
  await page.route('**/api/metrics/breakdown?*', route => {
    const query = new URL(route.request().url()).searchParams
    return route.fulfill({ json: {
      rows: query.get('slot') === String(peak) ? [{ value: '192.0.2.12', upload: 1024, download: 8192 }] : [],
      hasMore: false,
    } })
  })
  await page.goto('/')
  await page.getByRole('button', { name: '查看历史' }).click()
  await page.getByRole('button', { name: '最近 30 天' }).click()
  const axis = page.getByRole('group', { name: '逐日日期轴' })
  await expect(axis.locator(':scope > *')).toHaveCount(30)
  await expect(axis.locator(':scope > :first-child')).toHaveText('10/24')
  await expect(axis.locator(':scope > :last-child')).toHaveText('11/22')
  await expect(axis.getByText('10/24', { exact: true })).toHaveAttribute('title', /无记录/)
  const chart = page.getByRole('button', { name: '选择图表时段' })
  const box = (await chart.boundingBox())!
  await chart.click({ position: { x: box.width * 12.5 / 30, y: 25 } })
  const details = page.getByRole('region', { name: '所选时段流量明细' })
  await expect(details).toContainText('2026/11/05')
  await expect(details).toContainText('9.0 KiB')
  await expect(page.getByRole('region', { name: '来源 IP用量排行' })).toContainText('192.0.2.12')
  await expect(axis.getByRole('button', { name: /2026\/11\/05/ })).toHaveAttribute('aria-pressed', 'true')
  await chart.press('ArrowRight')
  await expect(details).toContainText('2026/11/06')

  await page.setViewportSize({ width: 390, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await expect(page.getByText('横向滚动查看全部日期')).toBeVisible()
  await expect.poll(() => axis.locator('[aria-pressed="true"]').evaluate(tick => {
    const scroller = tick.closest('.traffic-date-scroll')!.getBoundingClientRect()
    const bounds = tick.getBoundingClientRect()
    return bounds.left >= scroller.left && bounds.right <= scroller.right
  })).toBe(true)
  await axis.getByRole('button', { name: /2026\/11\/22/ }).click()
  await expect(details).toContainText('2026/11/22')
  const hourRequest = page.waitForRequest(request => {
    const url = new URL(request.url())
    return url.pathname.endsWith('/metrics/history') && url.searchParams.get('granularity') === 'hour' &&
      url.searchParams.get('from') === '2026-11-22' && url.searchParams.get('to') === '2026-11-22'
  })
  await details.getByRole('button', { name: '当天小时明细' }).click()
  await hourRequest
  await expect(page.getByLabel('统计粒度')).toHaveValue('hour')
})
