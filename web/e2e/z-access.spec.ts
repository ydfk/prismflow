import { test, expect } from '@playwright/test'
import { resolve } from 'node:path'

test('公网地址分工、外部 TLS 创建及反代配置弹窗',async({page})=>{
 await page.goto('/');await page.getByLabel('用户名',{exact:true}).fill('admin');await page.getByLabel('密码',{exact:true}).fill('PrismFlow-E2E-password-123');await page.getByRole('button',{name:'登录工作台',exact:true}).click()
 await page.getByRole('button',{name:'域名与证书',exact:true}).click()
 // 保持本地测试来源，测试真实路径检测，同时单独覆盖域名建议的填写行为。
 await page.getByLabel('基础域名',{exact:true}).fill('example.com');await page.getByRole('button',{name:'生成建议地址',exact:true}).click()
 await expect(page.getByLabel('面板公网地址',{exact:true})).toHaveValue('https://panel.example.com');await expect(page.getByRole('button',{name:'反代配置与检查',exact:true})).toBeDisabled();await expect(page.getByText('有未保存修改，请先保存再生成反代配置。',{exact:true})).toBeVisible()
 await page.getByLabel('面板公网地址',{exact:true}).fill('http://127.0.0.1:18880');await page.getByLabel('公开订阅地址',{exact:true}).fill('http://127.0.0.1:18881');await page.getByLabel('XHTTP 默认 TLS 方式',{exact:true}).selectOption('external')
 await page.getByRole('button',{name:'保存访问设置',exact:true}).click();await expect(page.getByText('公网访问设置已保存，仅作为新建默认值',{exact:true})).toBeVisible()
 await page.screenshot({path:resolve('../docs/screenshots/access.png'),fullPage:true,animations:'disabled'})
 await page.getByRole('button',{name:'反代配置与检查',exact:true}).click();const modal=page.getByRole('dialog',{name:'反代配置与部署检查',exact:true});await expect(modal.getByText('根据已保存的地址生成。将所需规则合并到现有反代；Nginx 请替换证书路径。不会改写服务器文件。',{exact:true})).toBeVisible();await expect(modal.getByText(/无需反代；配置将跳过/)).toHaveCount(2)
 await modal.getByRole('button',{name:'检查公网访问',exact:true}).click();await expect(modal.getByText('管理面板 · 正常')).toBeVisible();await expect(modal.getByText('订阅与网站 · 正常')).toBeVisible();await page.keyboard.press('Escape')
 await page.getByRole('button',{name:'协议入口',exact:true}).click();await page.getByRole('button',{name:'快速创建',exact:true}).click();const quick=page.getByRole('dialog',{name:'快速创建',exact:true});await quick.getByRole('radio',{name:/VLESS \+ XHTTP/}).check()
 await expect(quick.getByLabel('TLS 证书',{exact:true})).toHaveCount(0);await expect(quick.getByLabel('公网端口',{exact:true})).toHaveValue('443');await quick.getByRole('button',{name:'转为普通创建',exact:true}).click()
 const editor=page.getByRole('dialog',{name:'普通创建',exact:true});await expect(editor.getByLabel('TLS 终止位置',{exact:true})).toHaveValue('external');await editor.getByLabel('名称',{exact:true}).fill('反代 XHTTP');await editor.getByRole('button',{name:'创建入口',exact:true}).click();await expect(editor).toBeHidden()
 await expect(page.getByRole('row').filter({hasText:'反代 XHTTP'})).toContainText('node.example.com:443')
 await page.getByRole('button',{name:'域名与证书',exact:true}).click();await page.getByRole('button',{name:'反代配置与检查',exact:true}).click();await modal.getByRole('button',{name:'Nginx',exact:true}).click();await expect(modal.getByLabel('nginx 配置')).toContainText('proxy_request_buffering off')
 await page.screenshot({path:resolve('../docs/screenshots/reverse-proxy.png'),fullPage:true,animations:'disabled'});await page.setViewportSize({width:390,height:844});expect(await modal.evaluate(e=>e.scrollWidth<=e.clientWidth)).toBe(true);await page.screenshot({path:resolve('../docs/screenshots/mobile-access.png'),fullPage:true,animations:'disabled'})
})
