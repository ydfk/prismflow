export type Endpoint = {
  id: string; name: string; protocol: string; host: string; port: number; enabled: boolean; native?: boolean; nativeConfig?: string
  tlsMode?: string; publicPort?: number; transport: string; path: string; security: string; certificateId: string; serverName: string
  realityTarget: string; privateKey: string; publicKey: string; shortId: string; password: string; masquerade: boolean
}
export type User = { id: string; name: string; enabled: boolean; endpointIds: string[]; createdAt: string }
export type Certificate = { names?: string[]; id: string; domain: string; expiresAt: string; method: string; staging: boolean; renew: boolean; lastError: string; engine?: string; certPath?: string; keyPath?: string }
export type Website = { disabled: boolean; title: string; description: string; template: string; bundle: string }
export type AccessSettings = {baseDomain:string;panelUrl:string;xhttpTls:string}; export type State = { access: AccessSettings; revision: number; defaultHost: string; defaultCertificateId: string; endpoints: Endpoint[]; users: User[]; certificates: Certificate[]; website: Website; publicUrl: string; retentionDays: number; traffic: TrafficSettings }
export type Point = { time: number; protocol: string; upload: number; download: number }
export type Snapshot = { traffic: TrafficSummary; state: State; appliedRevision: number; core: { available: boolean; running: boolean; targetVersion: string }; metrics: { online: boolean; rates: Point[]; sampledAt: string; intervalSeconds: number } }
export type Protocol = { id: string; name: string; summary: string; detail: string; transports: string[]; securities: string[]; network: string }
export type Job = { id: string; domain: string; status: string; message: string; createdAt: string }
export type Page = 'overview' | 'endpoints' | 'users' | 'certificates' | 'settings'
export type PageProps = { snapshot: Snapshot; refresh: () => Promise<void>; notify: (message: string) => void }






export type TrafficSettings={maxBytes:number;resetDay:number;timezone:string;interface:string;outboundOnly:boolean}
export type TrafficSummary={upload:number;download:number;used:number;total:number;resetAt:number;interface:string;available:boolean;message:string;sampledAt:string}
