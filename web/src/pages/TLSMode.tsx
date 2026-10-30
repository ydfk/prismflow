import { Field } from '../ui'
export default function TLSMode({value,onChange,port,onPort}:{value:string;onChange:(value:string)=>void;port:number;onPort:(port:number)=>void}){
 return <div className="form-grid"><Field label="TLS 终止位置" hint="反代负责 TLS 时，Mihomo 仅在 127.0.0.1 的随机端口接收明文 XHTTP；客户端仍使用公网 TLS。反代需要与 PrismFlow 在同一主机网络中。"><select value={value} onChange={e=>onChange(e.target.value)}><option value="external">Caddy / Nginx · 使用反代证书</option><option value="core">Mihomo · 绑定节点证书</option></select></Field>{value==='external'&&<Field label="公网端口" hint="客户端连接反代的端口，通常为 443；不会修改 Mihomo 的本机监听端口。"><input type="number" min={1} max={65535} required value={port||443} onChange={e=>onPort(Number(e.target.value))}/></Field>}</div>
}
