import type { ReactElement } from 'react'
import { Box, Button, Typography } from '@mui/material'

interface EmptyStateProps {
  icon?: ReactElement
  title: string
  description?: string
  actionLabel?: string
  onAction?: () => void
}

/** 列表空状态：图标 + 文案 + 可选引导按钮 */
export const EmptyState = ({ icon, title, description, actionLabel, onAction }: EmptyStateProps) => {
  return (
    <Box sx={{ py: 6, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1 }}>
      {icon && (
        <Box sx={{ color: 'text.disabled', mb: 1, '& svg': { fontSize: 56 } }}>
          {icon}
        </Box>
      )}
      <Typography variant="subtitle1">{title}</Typography>
      {description && (
        <Typography variant="body2" color="text.secondary">
          {description}
        </Typography>
      )}
      {actionLabel && onAction && (
        <Button variant="contained" size="small" sx={{ mt: 2 }} onClick={onAction}>
          {actionLabel}
        </Button>
      )}
    </Box>
  )
}
