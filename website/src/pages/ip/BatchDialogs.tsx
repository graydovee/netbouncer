import { useEffect, useState, type ReactNode } from 'react'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from '@mui/material'
import { ipApi } from '../../api/ip'
import { errorMessage } from '../../api/client'
import type { BatchResult, IpGroup, IpNetAction } from '../../api/types'
import { actionLabel } from '../../utils/actions'

type Notify = (message: string, severity?: 'success' | 'error' | 'warning' | 'info') => void

interface BatchSetActionDialogProps {
  open: boolean
  ids: number[]
  onClose: () => void
  onMessage: Notify
  onSuccess: (result: BatchResult) => void
}

/** 批量设置选中IP的规则动作 */
export const BatchSetActionDialog = ({
  open,
  ids,
  onClose,
  onMessage,
  onSuccess,
}: BatchSetActionDialogProps) => {
  const [action, setAction] = useState<IpNetAction>('ban')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async () => {
    setLoading(true)
    try {
      const result = await ipApi.batchUpdateAction(ids, action)
      onSuccess(result)
      onClose()
    } catch (err) {
      onMessage(errorMessage(err, '批量设置失败'), 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <BatchDialogShell
      open={open}
      count={ids.length}
      title="批量设置行为"
      description="为选中的IP设置规则动作"
      loading={loading}
      onClose={onClose}
      onSubmit={() => void handleSubmit()}
      submitLabel="确认设置"
    >
      <FormControl fullWidth size="small">
        <InputLabel>选择行为</InputLabel>
        <Select
          value={action}
          label="选择行为"
          onChange={(event) => setAction(event.target.value as IpNetAction)}
        >
          {(['ban', 'allow'] as IpNetAction[]).map((item) => (
            <MenuItem key={item} value={item}>
              {actionLabel(item)}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
    </BatchDialogShell>
  )
}

interface BatchSetGroupDialogProps {
  open: boolean
  ids: number[]
  groups: IpGroup[]
  onClose: () => void
  onMessage: Notify
  onSuccess: (result: BatchResult) => void
}

/** 批量移动选中IP到指定组 */
export const BatchSetGroupDialog = ({
  open,
  ids,
  groups,
  onClose,
  onMessage,
  onSuccess,
}: BatchSetGroupDialogProps) => {
  const [groupId, setGroupId] = useState<number | ''>('')
  const [loading, setLoading] = useState(false)

  // 打开对话框时默认选中第一个组（组列表可能晚于组件挂载加载完成）
  useEffect(() => {
    if (open && groups.length > 0) {
      setGroupId(groups[0].id)
    }
  }, [open, groups])

  const handleSubmit = async () => {
    if (groupId === '') {
      onMessage('请选择要设置的组', 'warning')
      return
    }

    setLoading(true)
    try {
      const result = await ipApi.batchUpdateGroup(ids, groupId)
      onSuccess(result)
      onClose()
    } catch (err) {
      onMessage(errorMessage(err, '批量设置失败'), 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <BatchDialogShell
      open={open}
      count={ids.length}
      title="批量设置组"
      description="将选中的IP移动到指定组"
      loading={loading}
      onClose={onClose}
      onSubmit={() => void handleSubmit()}
      submitLabel="确认设置"
    >
      <FormControl fullWidth size="small">
        <InputLabel>选择组</InputLabel>
        <Select
          value={groupId}
          label="选择组"
          onChange={(event) => setGroupId(Number(event.target.value))}
        >
          {groups.map((group) => (
            <MenuItem key={group.id} value={group.id}>
              {group.name}
              {group.description && (
                <Typography variant="caption" sx={{ ml: 1, color: 'text.secondary' }}>
                  ({group.description})
                </Typography>
              )}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
    </BatchDialogShell>
  )
}

interface BatchDialogShellProps {
  open: boolean
  count: number
  title: string
  description: string
  loading: boolean
  onClose: () => void
  onSubmit: () => void
  submitLabel: string
  children: ReactNode
}

/** 批量对话框通用外壳 */
function BatchDialogShell({
  open,
  count,
  title,
  description,
  loading,
  onClose,
  onSubmit,
  submitLabel,
  children,
}: BatchDialogShellProps) {
  return (
    <Dialog
      open={open}
      onClose={() => {
        if (!loading) onClose()
      }}
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {description}（已选择 {count} 个IP）
        </Typography>
        {children}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          取消
        </Button>
        <Button onClick={onSubmit} color="primary" disabled={loading || count === 0}>
          {loading ? '设置中...' : submitLabel}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
