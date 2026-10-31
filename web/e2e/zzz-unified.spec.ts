import {test,expect} from '@playwright/test'
import {resolve} from 'node:path'

test('同域路径登录、订阅隔离与逐证书日志',async({page})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
 await page.goto('http://127.0.0.1:18881/manage/');await page.getByLabel('用户名',{exact:true}).fill('admin');await page.getByLabel('密码',{exact:true}).fill('PrismFlow-E2E-password-123');await page.getByRole('button',{name:'登录工作台',exact:true}).click();await expect(page.getByRole('heading',{name:'总览',exact:true})).toBeVisible()
 expect((await page.request.get('http://127.0.0.1:18881/api/state')).status()).toBe(404)
 await page.getByRole('button',{name:'域名与证书',exact:true}).click();await expect(page.getByLabel('统一访问地址',{exact:true})).toHaveValue('http://127.0.0.1:18881')
 await expect(page.getByRole('heading',{name:'申请任务',exact:true})).toHaveCount(0)
 const row=page.getByRole('row').filter({hasText:'localhost'}).last()
 await row.getByRole('button',{name:'查看证书日志 localhost',exact:true}).click();const logs=page.getByRole('dialog',{name:'localhost · 证书操作记录',exact:true});await expect(logs).toContainText('导入证书');await expect(logs).toContainText('成功');await page.keyboard.press('Escape')
 await row.getByRole('button',{name:'替换证书 localhost',exact:true}).hover();await expect(row.getByRole('button',{name:'替换证书 localhost',exact:true})).toHaveAttribute('title',/PEM/)
 await page.getByRole('button',{name:'申请证书',exact:true}).click();const form=page.getByRole('dialog',{name:'申请 ACME 证书',exact:true});await form.getByText('测试 CA（仅用于排查申请流程）',{exact:true}).click();await expect(form.getByRole('checkbox',{name:/使用测试 CA/})).not.toBeChecked();await form.getByRole('checkbox',{name:/使用测试 CA/}).check();await page.keyboard.press('Escape');await page.getByRole('button',{name:'申请证书',exact:true}).click();await form.getByText('测试 CA（仅用于排查申请流程）',{exact:true}).click();await expect(form.getByRole('checkbox',{name:/使用测试 CA/})).not.toBeChecked();await page.keyboard.press('Escape')
 // 模拟既有自动证书，不发起真实 CA 申请；检查来源对应的操作。
 await page.route('**/api/state',async route=>{const response=await route.fetch();const data=await response.json();data.state.certificates[0].method='http';data.state.certificates[0].engine='acmesh';data.state.certificates[0].renew=true;data.state.certificates[0].staging=true;await route.fulfill({response,json:data})})
 await page.reload()
 await expect(page.getByRole('heading',{name:'总览',exact:true})).toBeVisible();await page.getByRole('button',{name:'域名与证书',exact:true}).click()
 await expect(page.getByRole('button',{name:'替换证书 localhost',exact:true})).toHaveCount(0);await expect(page.getByText('测试 CA · 不受信任',{exact:true})).toBeVisible();await expect(page.getByRole('cell').filter({hasText:'acme.sh 自动申请'})).toBeVisible()
 await expect(page.getByRole('button',{name:'申请正式证书 localhost',exact:true})).toBeVisible();await page.getByRole('button',{name:'刷新或续期 localhost',exact:true}).click();await expect(page.getByRole('dialog',{name:'立即续期证书',exact:true})).toBeVisible();await page.getByRole('button',{name:'取消',exact:true}).click()
 await page.screenshot({path:resolve('../docs/screenshots/unified-certificates.png'),fullPage:true,animations:'disabled'});await page.setViewportSize({width:390,height:844});await page.screenshot({path:resolve('../docs/screenshots/mobile-unified-certificates.png'),fullPage:true,animations:'disabled'})
 await page.unroute('**/api/state');await page.setViewportSize({width:1440,height:1000});await page.getByLabel('管理入口路径',{exact:true}).fill('/control-test');await page.getByRole('button',{name:'保存访问设置',exact:true}).click();await page.waitForURL('**/control-test/');await expect(page.getByRole('heading',{name:'总览',exact:true})).toBeVisible()
 expect((await page.request.get('http://127.0.0.1:18881/control-test/api/status')).status()).toBe(200);expect((await page.request.get('http://127.0.0.1:18881/manage/api/status')).status()).toBe(404)
 expect(errors).toEqual([])
})
