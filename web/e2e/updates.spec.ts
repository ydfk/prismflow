import {test,expect} from '@playwright/test'
import {resolve} from 'node:path'
test('原生面板版本检查、更新确认与自动更新',async({page})=>{
 await page.goto('/');await page.getByLabel('用户名',{exact:true}).fill('admin');await page.getByLabel('密码',{exact:true}).fill('PrismFlow-E2E-password-123');await page.getByRole('button',{name:'登录工作台',exact:true}).click();await expect(page.getByRole('heading',{name:'总览',exact:true})).toBeVisible()
 const status={current:'0.1.0',mode:'native',native:true,auto:false,pending:false,error:'',release:{version:'',available:false,url:'',checkedAt:''}}
 await page.route('**/api/updates',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(status)}))
 await page.route('**/api/updates/check',route=>{status.release={version:'v0.2.0',available:true,url:'https://github.com/ydfk/prismflow/releases/tag/v0.2.0',checkedAt:new Date().toISOString()};return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(status.release)})})
 let requested=0;await page.route('**/api/updates/install',route=>{requested++;status.pending=true;return route.fulfill({status:202,contentType:'application/json',body:'{"message":"queued"}'})})
 await page.route('**/api/updates/automatic',route=>{status.auto=route.request().postDataJSON().enabled;return route.fulfill({status:200,contentType:'application/json',body:'{"ok":true}'})})
 await page.getByRole('button',{name:'系统设置',exact:true}).click();const panel=page.locator('.panel-updates');await expect(panel.getByText('0.1.0',{exact:true})).toBeVisible();await expect(panel.getByRole('button',{name:'更新面板',exact:true})).toBeDisabled()
 await panel.getByRole('button',{name:'检查面板版本',exact:true}).click();await expect(panel.getByRole('link',{name:'v0.2.0',exact:true})).toBeVisible();await expect(panel.getByRole('button',{name:'更新面板',exact:true})).toBeEnabled()
 await panel.getByRole('button',{name:'更新面板',exact:true}).click();let modal=page.getByRole('dialog',{name:'更新 PrismFlow 面板',exact:true});await expect(modal).toContainText('v0.2.0');await modal.getByRole('button',{name:'取消',exact:true}).click();expect(requested).toBe(0)
 const automatic=panel.getByRole('checkbox');await automatic.click();modal=page.getByRole('dialog',{name:'启用自动更新',exact:true});await expect(modal).toContainText('每小时');await modal.getByRole('button',{name:'启用自动更新',exact:true}).click();await expect(modal).toBeHidden();await expect(automatic).toBeChecked();await automatic.click();await expect(automatic).not.toBeChecked()
 await panel.scrollIntoViewIfNeeded();await panel.screenshot({path:resolve('../docs/screenshots/updates.png'),animations:'disabled'})
 await panel.getByRole('button',{name:'更新面板',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'确认更新',exact:true}).click();await expect(page.getByRole('dialog')).toBeHidden();expect(requested).toBe(1);await expect(panel.getByRole('button',{name:'已提交更新',exact:true})).toBeDisabled()
 await page.setViewportSize({width:390,height:844});await panel.scrollIntoViewIfNeeded();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await panel.screenshot({path:resolve('../docs/screenshots/mobile-updates.png'),animations:'disabled'})
})
