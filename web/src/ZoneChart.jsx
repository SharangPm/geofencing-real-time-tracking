import React from'react';
import{ResponsiveContainer,PieChart,Pie,Tooltip,Cell}from'recharts';
const colors=['#3974d5','#e35462','#ae78dd','#2ca985'];
export default function ZoneChart({counts,total}){return <><div className="chart-box"><ResponsiveContainer width="100%" height="190"><PieChart><Pie data={counts} dataKey="count" nameKey="name" innerRadius={57} outerRadius={82} paddingAngle={4}>{counts.map((v,i)=><Cell key={i} fill={colors[i]}/>)}</Pie><Tooltip/></PieChart></ResponsiveContainer><div className="chart-center"><b>{total}</b><small>zones</small></div></div><div className="legend">{counts.map((c,i)=><div key={c.name}><i style={{background:colors[i]}}/>{c.name}<b>{c.count}</b></div>)}</div></>}
