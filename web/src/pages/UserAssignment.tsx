import type { User } from '../types'
export default function UserAssignment({users,value,onChange}:{users:User[];value:string[];onChange:(value:string[])=>void}){
 return <fieldset className="assignment compact-assignment"><legend>分配用户</legend>{users.length?users.map(u=><label key={u.id}><input type="checkbox" checked={value.includes(u.id)} onChange={e=>onChange(e.target.checked?[...value,u.id]:value.filter(id=>id!==u.id))}/>{u.name}</label>):<small>还没有用户，可创建后再分配。</small>}</fieldset>
}
