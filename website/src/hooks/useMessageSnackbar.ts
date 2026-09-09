import { useState } from 'react'

export interface SnackbarState {
  open: boolean
  message: string
  severity: 'success' | 'error' | 'warning' | 'info'
}

/** 消息提示 Hook：showMessage(message, severity) / hideMessage() */
export const useMessageSnackbar = () => {
  const [snackbar, setSnackbar] = useState<SnackbarState>({
    open: false,
    message: '',
    severity: 'success',
  })

  const showMessage = (message: string, severity: SnackbarState['severity'] = 'success') => {
    setSnackbar({ open: true, message, severity })
  }

  const hideMessage = () => {
    setSnackbar((prev) => ({ ...prev, open: false }))
  }

  return { snackbar, showMessage, hideMessage }
}
