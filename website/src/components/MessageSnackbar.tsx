import { Alert, Snackbar } from '@mui/material'
import type { SnackbarState } from '../hooks/useMessageSnackbar'

interface MessageSnackbarProps {
  snackbar: SnackbarState
  onClose: () => void
}

/** 右下角消息提示条 */
export const MessageSnackbar = ({ snackbar, onClose }: MessageSnackbarProps) => {
  return (
    <Snackbar
      open={snackbar.open}
      autoHideDuration={4000}
      onClose={onClose}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
    >
      <Alert onClose={onClose} severity={snackbar.severity} sx={{ width: '100%' }}>
        {snackbar.message}
      </Alert>
    </Snackbar>
  )
}
