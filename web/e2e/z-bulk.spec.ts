import {test,expect} from '@playwright/test'
import {resolve} from 'node:path'

test('非默认 HTTPS 端口与混合 TLS 批量创建',async({page})=>{
 await page.goto('/');await page.getByLabel('用户名',{exact:true}).fill('admin');await page.getByLabel('密码',{exact:true}).fill('PrismFlow-E2E-password-123');await page.getByRole('button',{name:'登录工作台',exact:true}).click();await expect(page.getByRole('heading',{name:'总览',exact:true})).toBeVisible()
 const response=await page.request.get('/api/state');const original=(await response.json()).state;const headers={'X-CSRF-Token':response.headers()['x-csrf-token']};let added:string[]=[]
 try{
  const saved=await page.request.put('/api/settings',{headers,data:{publicUrl:'https://localhost:8443',defaultHost:'localhost',access:{...original.access,panelUrl:'https://localhost:8443',xhttpTls:'external'}}});expect(saved.status()).toBe(200)
  await page.reload();await page.getByRole('button',{name:'协议入口',exact:true}).click();await page.getByRole('button',{name:'快速创建',exact:true}).click();let dialog=page.getByRole('dialog',{name:'快速创建',exact:true})
  await dialog.getByRole('button',{name:'清空协议',exact:true}).click();await dialog.getByRole('button',{name:'创建入口',exact:true}).click();await expect(dialog.getByRole('alert')).toContainText('至少选择一个协议')
  await dialog.getByRole('checkbox',{name:/VLESS \+ XHTTP/}).check();await expect(dialog.getByLabel(/^(XHTTP )?公网端口$/)).toHaveValue('8443');await expect(dialog.getByLabel('TLS 证书',{exact:true})).toHaveCount(0)
  await dialog.getByRole('button',{name:'转为普通创建',exact:true}).click();const editor=page.getByRole('dialog',{name:'普通创建',exact:true});await expect(editor.getByLabel(/^(XHTTP )?公网端口$/)).toHaveValue('8443');await page.keyboard.press('Escape')
  await page.getByRole('button',{name:'快速创建',exact:true}).click();dialog=page.getByRole('dialog',{name:'快速创建',exact:true});await dialog.getByRole('button',{name:'全选协议',exact:true}).click();await expect(dialog.locator('.protocol-choices input:checked')).toHaveCount(4);await expect(dialog.getByRole('button',{name:'转为普通创建',exact:true})).toBeDisabled();await expect(dialog.getByLabel('TLS 证书',{exact:true})).toBeVisible();await expect(dialog.getByLabel(/^(XHTTP )?公网端口$/)).toHaveValue('8443')
  await expect.poll(async()=>await dialog.locator('.protocol-choices .mono').allTextContents()).toEqual(expect.arrayContaining([expect.stringMatching(/^\d{5}$/)]))
  const ports=await dialog.locator('.protocol-choices .mono').allTextContents();expect(new Set(ports).size).toBe(4);expect(ports.every(p=>/^\d{5}$/.test(p))).toBe(true)
  await page.screenshot({path:resolve('../docs/screenshots/bulk-create.png'),fullPage:true,animations:'disabled'});await page.setViewportSize({width:390,height:844});await page.screenshot({path:resolve('../docs/screenshots/mobile-bulk-create.png'),fullPage:true,animations:'disabled'});expect(await dialog.evaluate(e=>e.scrollWidth<=e.clientWidth)).toBe(true);await page.setViewportSize({width:1440,height:1000})
  await dialog.getByRole('checkbox',{name:/VLESS \+ REALITY/}).uncheck();await dialog.getByRole('checkbox',{name:/^Hysteria2/}).uncheck();await dialog.getByRole('checkbox',{name:'测试设备',exact:true}).check();await dialog.getByRole('button',{name:'创建 2 个入口',exact:true}).click();await expect(dialog).toBeHidden({timeout:20000})
  const snapshot=await (await page.request.get('/api/state')).json();const fresh=snapshot.state.endpoints.filter((e:{id:string})=>!original.endpoints.some((old:{id:string})=>old.id===e.id));added=fresh.map((e:{id:string})=>e.id);expect(fresh).toHaveLength(2);expect(snapshot.core.running).toBe(true)
  const xhttp=fresh.find((e:{transport:string})=>e.transport==='xhttp');const anytls=fresh.find((e:{protocol:string})=>e.protocol==='anytls');expect(xhttp.publicPort).toBe(8443);expect(xhttp.certificateId).toBe('');expect(anytls.certificateId).toBe(original.defaultCertificateId)
  const share=await (await page.request.get(`/api/users/${original.users[0].id}/subscription`)).json();expect(share.url).toContain('https://localhost:8443/s/');expect(share.links).toContain('localhost:8443')
  const config=await (await page.request.get('/api/access/config')).json();expect(config.caddy).toContain('https://localhost:8443 {');expect(config.nginx).toContain('listen 8443 ssl http2;')
 }finally{
  for(const id of added){const removed=await page.request.delete(`/api/endpoints/${id}`,{headers});expect(removed.status()).toBe(200)}
  const restored=await page.request.put('/api/settings',{headers,data:{publicUrl:original.publicUrl,defaultHost:original.defaultHost,access:original.access}});expect(restored.status()).toBe(200)
 }
})
