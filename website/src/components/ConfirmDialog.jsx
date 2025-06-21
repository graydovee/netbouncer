import { useState } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Typography,
  Box,
} from '@mui/material';
import {
  Warning as WarningIcon,
  Info as InfoIcon,
  Error as ErrorIcon,
  Help as HelpIcon,
} from '@mui/icons-material';

// 确认对话框Hook
export const useConfirmDialog = () => {
  const [confirmDialog, setConfirmDialog] = useState({
    open: false,
    title: '',
    message: '',
    type: 'warning', // 'warning', 'info', 'error', 'question'
    onConfirm: null,
    onCancel: null,
  });

  const showConfirm = (title, message, type = 'warning', onConfirm, onCancel) => {
    setConfirmDialog({
      open: true,
      title,
      message,
      type,
      onConfirm,
      onCancel,
    });
  };

  const hideConfirm = () => {
    setConfirmDialog(prev => ({ ...prev, open: false }));
  };

  const handleConfirm = () => {
    if (confirmDialog.onConfirm) {
      confirmDialog.onConfirm();
    }
    hideConfirm();
  };

  const handleCancel = () => {
    if (confirmDialog.onCancel) {
      confirmDialog.onCancel();
    }
    hideConfirm();
  };

  return {
    confirmDialog,
    showConfirm,
    hideConfirm,
    handleConfirm,
    handleCancel,
  };
};

// 确认对话框组件
export const ConfirmDialog = ({ confirmDialog, onConfirm, onCancel }) => {
  const getIcon = (type) => {
    switch (type) {
      case 'warning':
        return <WarningIcon color="warning" sx={{ fontSize: 40 }} />;
      case 'error':
        return <ErrorIcon color="error" sx={{ fontSize: 40 }} />;
      case 'info':
        return <InfoIcon color="info" sx={{ fontSize: 40 }} />;
      case 'question':
        return <HelpIcon color="primary" sx={{ fontSize: 40 }} />;
      default:
        return <WarningIcon color="warning" sx={{ fontSize: 40 }} />;
    }
  };

  const getConfirmButtonColor = (type) => {
    switch (type) {
      case 'warning':
        return 'warning';
      case 'error':
        return 'error';
      case 'info':
        return 'info';
      case 'question':
        return 'primary';
      default:
        return 'warning';
    }
  };

  const getConfirmButtonText = (type) => {
    switch (type) {
      case 'warning':
        return '确认';
      case 'error':
        return '确认';
      case 'info':
        return '确定';
      case 'question':
        return '是';
      default:
        return '确认';
    }
  };

  return (
    <Dialog
      open={confirmDialog.open}
      onClose={onCancel}
      maxWidth="sm"
      fullWidth
      PaperProps={{
        sx: {
          borderRadius: 2,
        }
      }}
    >
      <DialogTitle sx={{ pb: 1 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          {getIcon(confirmDialog.type)}
          <Typography variant="h6" component="div">
            {confirmDialog.title}
          </Typography>
        </Box>
      </DialogTitle>
      <DialogContent sx={{ pt: 0, pb: 2 }}>
        <Typography variant="body1" sx={{ whiteSpace: 'pre-line' }}>
          {confirmDialog.message}
        </Typography>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 3 }}>
        <Button
          onClick={onCancel}
          variant="outlined"
          sx={{ minWidth: 80 }}
        >
          取消
        </Button>
        <Button
          onClick={onConfirm}
          variant="contained"
          color={getConfirmButtonColor(confirmDialog.type)}
          sx={{ minWidth: 80 }}
        >
          {getConfirmButtonText(confirmDialog.type)}
        </Button>
      </DialogActions>
    </Dialog>
  );
}; 