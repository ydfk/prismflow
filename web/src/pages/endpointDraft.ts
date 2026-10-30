import type { Endpoint, Snapshot } from '../types'
export type EndpointDraft = Endpoint & { userIds: string[] }
export const quickChoices = [
 {id:'reality',name:'VLESS + REALITY + Vision',protocol:'vless',transport:'tcp',security:'reality'},
 {id:'xhttp',name:'VLESS + XHTTP + TLS',protocol:'vless',transport:'xhttp',security:'tls'},
 {id:'anytls',name:'AnyTLS',protocol:'anytls',transport:'tcp',security:'tls'},
 {id:'hysteria2',name:'Hysteria2',protocol:'hysteria2',transport:'quic',security:'tls'},
]
export function newDraft(snapshot:Snapshot):EndpointDraft {
 const certificateId=snapshot.state.defaultCertificateId||snapshot.state.certificates[0]?.id||''
 return {id:'',name:'',protocol:'vless',transport:'tcp',security:'reality',host:snapshot.state.defaultHost||'',port:0,enabled:true,path:'/prismflow',certificateId,serverName:'www.microsoft.com',realityTarget:'www.microsoft.com:443',privateKey:'',publicKey:'',shortId:'',password:'',masquerade:false,native:false,nativeConfig:'',userIds:[]}
}
export function nativeTemplate(protocol:string):string {
 const password=Array.from(crypto.getRandomValues(new Uint8Array(18)),b=>b.toString(16).padStart(2,'0')).join('')
 if(['http','socks','mixed'].includes(protocol))return `listen: 127.0.0.1\nusers:\n  - username: prismflow\n    password: ${password}\n`
 if(protocol==='tunnel')return 'listen: 127.0.0.1\nnetwork: [tcp, udp]\ntarget: 127.0.0.1:8080\n'
 if(protocol==='tun')return 'device: prismflow-tun\nstack: mixed\ninet4-address: [198.18.0.1/30]\nauto-route: false\n'
 if(protocol==='redir'||protocol==='tproxy')return 'listen: 127.0.0.1\n'
 return '# 填写此监听器的 Mihomo 原生参数；认证、TLS 等由此处配置。\nlisten: 127.0.0.1\n'
}
