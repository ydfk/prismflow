import {test,expect} from '@playwright/test'
import {resolve} from 'node:path'

test('帮助浮出不挤动输入，轮询与遮罩拖选不丢失编辑',async({page})=>{
 await page.goto('/');await page.getByLabel('用户名',{exact:true}).fill('admin');await page.getByLabel('密码',{exact:true}).fill('PrismFlow-E2E-password-123');await page.getByRole('button',{name:'登录工作台',exact:true}).click()
 await page.getByRole('button',{name:'协议入口',exact:true}).click();await page.getByRole('button',{name:'普通创建',exact:true}).click()
 const dialog=page.getByRole('dialog',{name:'普通创建',exact:true});const input=dialog.getByLabel('名称',{exact:true})
 await input.fill('保留编辑中的草稿');const address=dialog.getByLabel('节点连接地址',{exact:true});const before=await address.boundingBox()
 await dialog.getByRole('button',{name:'解释 节点连接地址',exact:true}).click();const note=dialog.getByRole('note');await expect(note).toContainText('写入订阅');await expect(note).toBeVisible();expect(await address.boundingBox()).toEqual(before)
 await page.screenshot({path:resolve('../docs/screenshots/help-popover.png'),fullPage:true,animations:'disabled'})
 await page.keyboard.press('Escape');await expect(note).toBeHidden();await expect(dialog).toBeVisible()
 await dialog.getByRole('button',{name:'解释 节点连接地址',exact:true}).click();await expect(note).toBeVisible()
 await dialog.evaluate(e=>e.dispatchEvent(new Event('scroll')));await expect(note).toBeHidden();await expect(dialog).toBeVisible()
 await page.mouse.click(2,2);await expect(dialog).toBeVisible()
 const box=await input.boundingBox();await page.mouse.move(box!.x+20,box!.y+10);await page.mouse.down();await page.mouse.move(2,2);await page.mouse.up();await expect(dialog).toBeVisible()
 // 状态每 5 秒刷新，确认刷新不会重建编辑器或清空草稿。
 await page.waitForTimeout(5500);await expect(input).toHaveValue('保留编辑中的草稿')
 await page.setViewportSize({width:390,height:844});await dialog.getByRole('button',{name:'解释 节点连接地址',exact:true}).click();await expect(note).toBeVisible()
 expect(await note.evaluate(e=>{const r=e.getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight})).toBe(true)
 await page.screenshot({path:resolve('../docs/screenshots/mobile-help-popover.png'),fullPage:true,animations:'disabled'})
 await page.keyboard.press('Escape');await expect(dialog).toBeVisible();await dialog.getByRole('button',{name:'关闭弹窗',exact:true}).click();await expect(dialog).toBeHidden()
})
