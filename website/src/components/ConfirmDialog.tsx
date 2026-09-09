/* eslint-disable react-refresh/only-export-components */
import { useState } from 'react'
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from '@mui/material'
import {
  Error as ErrorIcon,
  Help as HelpIcon,
  Info as InfoIcon,
  Warning as WarningIcon,
} from '@mui/icons-material'

export type ConfirmType = 'warning' | 'info' | 'error' | 'question'

export interface ConfirmState {
  open: boolean
  title: string
  message: string
  type: ConfirmType
  onConfirm: (() => void) | null
}

const initialConfirmState: ConfirmState = {
  open: false,
  title: '',
  message: '',
  type: 'warning',
  onConfirm: null,
}

interface ConfirmAction {
  /** 展示确认框，用户确认后执行 onConfirm */
  showConfirm: (
    title: string,
    message: string,
    type: ConfirmType,
    onConfirm: () => void,
  ) => void
  confirmState: ConfirmState
  handleConfirm: () => void
  handleCancel: () => void
}

/** 危险操作确认 Hook，配合 <ConfirmDialog /> 使用 */
export const useConfirmDialog = (): ConfirmAction => {
  const [confirmState, setConfirmState] = useState<ConfirmState>(initialConfirmState)

  const showConfirm = (
    title: string,
    message: string,
    type: ConfirmType,
    onConfirm: () => void,
  ) => {
    setConfirmState({ open: true, title, message, type, onConfirm })
  }

  const handleConfirm = () => {
    confirmState.onConfirm?.()
    setConfirmState(initialConfirmState)
  }

  const handleCancel = () => {
    setConfirmState(initialConfirmState)
  }

  return { showConfirm, confirmState, handleConfirm, handleCancel }
}

const typeIcon = (type: ConfirmType) => {
  switch (type) {
    case 'error':
      return <ErrorIcon color="error" sx={{ fontSize: 40 }} />
    case 'info':
      return <InfoIcon color="info" sx={{ fontSize: 40 }} />
    case 'question':
      return <HelpIcon color="primary" sx={{ fontSize: 40 }} />
    default:
      return <WarningIcon color="warning" sx={{ fontSize: 40 }} />
  }
}

const typeButtonColor = (type: ConfirmType) => {
  switch (type) {
    case 'error':
      return 'error' as const
    case 'info':
      return 'info' as const
    case 'question':
      return 'primary' as const
    default:
      return 'warning' as const
  }
}

interface ConfirmDialogProps {
  confirmState: ConfirmState
  onConfirm: () => void
  onCancel: () => void
}

/** 危险操作确认对话框 */
export const ConfirmDialog = ({ confirmState, onConfirm, onCancel }: ConfirmDialogProps) => {
  return (
    <Dialog open={confirmState.open} onClose={onCancel} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ pb: 1 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          {typeIcon(confirmState.type)}
          <Typography variant="h6" component="div">
            {confirmState.title}
          </Typography>
        </Box>
      </DialogTitle>
      <DialogContent sx={{ pt: 0, pb: 2 }}>
        <Typography variant="body1" sx={{ whiteSpace: 'pre-line' }}>
          {confirmState.message}
        </Typography>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 3 }}>
        <Button onClick={onCancel} variant="outlined" sx={{ minWidth: 80 }}>
          取消
        </Button>
        <Button
          onClick={onConfirm}
          variant="contained"
          color={typeButtonColor(confirmState.type)}
          sx={{ minWidth: 80 }}
        >
          确认
        </Button>
      </DialogActions>
    </Dialog>
  )
}
