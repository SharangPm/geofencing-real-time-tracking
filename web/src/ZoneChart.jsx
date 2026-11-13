
import React from 'react';
import {
  ResponsiveContainer,
  PieChart,
  Pie,
  Tooltip,
  Cell
} from 'recharts';

const colors = ['#3974d5', '#e35462', '#ae78dd', '#2ca985'];

export default function ZoneChart({ counts, total }) {
  return (
    <div style={{ width: '100%' }}>
      <div
        style={{
          position: 'relative',
          width: '100%',
          height: 190
        }}
      >
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={counts}
              dataKey="count"
              nameKey="name"
              cx="50%"
              cy="50%"
              innerRadius={55}
              outerRadius={78}
              paddingAngle={3}
              stroke="none"
            >
              {counts.map((item, index) => (
                <Cell
                  key={item.name}
                  fill={colors[index % colors.length]}
                />
              ))}
            </Pie>
            <Tooltip />
          </PieChart>
        </ResponsiveContainer>

        <div
          style={{
            position: 'absolute',
            top: '50%',
            left: '50%',
            transform: 'translate(-50%, -50%)',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            pointerEvents: 'none'
          }}
        >
          <b style={{ fontSize: 25, lineHeight: 1.2 }}>{total}</b>
          <small>{total === 1 ? 'zone' : 'zones'}</small>
        </div>
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: '12px 20px',
          padding: '0 4px'
        }}
      >
        {counts.map((item, index) => (
          <div
            key={item.name}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              fontSize: 12,
              minWidth: 0
            }}
          >
            <span style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
              <i
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  background: colors[index % colors.length],
                  flexShrink: 0
                }}
              />
              <span style={{ textTransform: 'capitalize' }}>
                {item.name}
              </span>
            </span>
            <b>{item.count}</b>
          </div>
        ))}
      </div>
    </div>
  );
}
