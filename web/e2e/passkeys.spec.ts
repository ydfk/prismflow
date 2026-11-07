import { expect, test } from '@playwright/test'
import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'

test('管理员登记、使用及撤销 Passkey，断言不可重放', async ({ page }) => {
  const origin = 'http://localhost:18880'
  const password = 'PrismFlow-E2E-password-123'
  const cdp = await page.context().newCDPSession(page)
  await cdp.send('WebAuthn.enable')
  await cdp.send('WebAuthn.addVirtualAuthenticator', { options: {
    protocol: 'ctap2', transport: 'internal', hasResidentKey: true,
    hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true,
  } })
  await page.goto(origin)
  const passwordLogin = async () => {
    await page.getByLabel('密码', { exact: true }).fill(password)
    const token = page.getByLabel('初始化令牌', { exact: true })
    if (await token.isVisible()) {
      const dir = await readFile(resolve('../.cache/e2e-path.txt'), 'utf8')
      await token.fill(await readFile(resolve(dir, 'setup.token'), 'utf8'))
    }
    await page.getByRole('button', { name: /创建管理员并登录|登录工作台/ }).click()
    await expect(page.getByRole('button', { name: '退出登录' })).toBeVisible()
  }
  const securitySettings = async () => {
    await page.getByRole('button', { name: '系统设置', exact: true }).click()
    await page.getByRole('tab', { name: '安全与备份' }).click()
  }
  const logout = async () => page.getByRole('button', { name: '退出登录' }).click()

  await passwordLogin()
  await securitySettings()
  await page.getByRole('button', { name: '添加 Passkey' }).click()
  const editor = page.getByRole('dialog', { name: '添加 Passkey' })
  await editor.getByLabel('Passkey 名称').fill('测试安全密钥')
  await editor.getByLabel('当前密码').fill(password)
  await editor.getByRole('button', { name: '登记 Passkey' }).click()
  await expect(page.getByText('Passkey 已添加', { exact: true })).toBeVisible()

  await logout()
  const login = page.getByRole('button', { name: '使用 Passkey 登录' })
  const submitted = page.waitForRequest(request => request.url() === origin + '/api/session/passkey' && request.method() === 'POST')
  await login.click()
  const assertion = (await submitted).postDataJSON()
  await expect(page.getByRole('button', { name: '退出登录' })).toBeVisible()
  const replay = await page.request.post(origin + '/api/session/passkey', { headers: { Origin: origin }, data: assertion })
  expect(replay.status()).toBe(400)
  const crossOrigin = await page.request.post(origin + '/api/session/passkey/options', { headers: { Origin: 'https://example.com' } })
  expect(crossOrigin.status()).toBe(403)

  await securitySettings()
  await expect(page.getByText(/^最近使用于 /)).toBeVisible()
  await page.getByRole('button', { name: '撤销 Passkey 测试安全密钥' }).click()
  await page.getByRole('button', { name: '确认撤销' }).click()
  await expect(page.getByText('尚未登记 Passkey')).toBeVisible()
  await logout()
  await login.click()
  await expect(page.getByRole('alert')).toContainText('当前域名没有可用的 Passkey')
  await passwordLogin()
})
