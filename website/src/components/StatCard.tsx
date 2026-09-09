import type { ReactElement } from 'react'
import { Card, CardContent, Box, Typography } from '@mui/material'

interface StatCardProps {
  icon: ReactElement
  label: string
  value: string
  sub?: string
  color?: 'primary' | 'secondary' | 'success' | 'error' | 'warning' | 'info'
}

/** 仪表盘概览统计卡片 */
export const StatCard = ({ icon, label, value, sub, color = 'primary' }: StatCardProps) => {
  return (
    <Card sx={{ height: '100%' }}>
      <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2, py: 2, '&:last-child': { pb: 2 } }}>
        <Box
          sx={(theme) => ({
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 48,
            height: 48,
            borderRadius: 2,
            color: `${color}.main`,
            backgroundColor: theme.palette.mode === 'dark'
              ? `color-mix(in srgb, ${theme.palette[color].main} 16%, transparent)`
              : `color-mix(in srgb, ${theme.palette[color].main} 10%, transparent)`,
          })}
        >
          {icon}
        </Box>
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="body2" color="text.secondary" noWrap>
            {label}
          </Typography>
          <Typography variant="h6" component="div" sx={{ fontWeight: 700, lineHeight: 1.3 }}>
            {value}
          </Typography>
          {sub && (
            <Typography variant="caption" color="text.secondary" noWrap display="block">
              {sub}
            </Typography>
          )}
        </Box>
      </CardContent>
    </Card>
  )
}
