import { useState } from 'react'
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
import type { IpGroup, IpNet, IpNetAction } from '../../api/types'
import { actionLabel } from '../../utils/actions'

interface ChangeGroupDialogProps {
  ipNet: IpNet | null
  groups: IpGroup[]
  onClose: () => void
  onMessage: (message: string, severity?: 'success' | 'error' | 'warning' | 'info') => void
  onSuccess: () => void
}

/** 修改单条IP规则所属组 */
export const ChangeGroupDialog = ({ ipNet, groups, onClose, onMessage, onSuccess }: ChangeGroupDialogProps) => {
  const [groupId, setGroupId] = useState<number | ''>('')
  const [loading, setLoading] = useState(false)
  const [initializedFor, setInitializedFor] = useState<number | null>(null)

  // 打开时以当前规则的组作为默认选中
  if (ipNet && initializedFor !== ipNet.id) {
    setInitializedFor(ipNet.id)
    setGroupId(ipNet.group?.id ?? '')
  }

  const handleSubmit = async () => {
    if (!ipNet || groupId === '') {
      onMessage('请选择新的组', 'warning')
      return
    }

    setLoading(true)
    try {
      await ipApi.updateGroup(ipNet.id, groupId)
      onMessage(`成功修改 ${ipNet.ip_net} 的所属组`)
      onSuccess()
      onClose()
    } catch (err) {
      onMessage(errorMessage(err, '修改失败'), 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={Boolean(ipNet)} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>修改IP所属组</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          当前IP: {ipNet?.ip_net}
        </Typography>
        <FormControl fullWidth size="small">
          <InputLabel>选择新的组</InputLabel>
          <Select
            value={groupId}
            label="选择新的组"
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
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} color="primary">
          取消
        </Button>
        <Button onClick={() => void handleSubmit()} color="primary" disabled={loading}>
          {loading ? '修改中...' : '修改'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}

interface ChangeActionDialogProps {
  ipNet: IpNet | null
  onClose: () => void
  onMessage: (message: string, severity?: 'success' | 'error' | 'warning' | 'info') => void
  onSuccess: () => void
}

/** 修改单条IP规则动作（禁用/允许） */
export const ChangeActionDialog = ({ ipNet, onClose, onMessage, onSuccess }: ChangeActionDialogProps) => {
  const [action, setAction] = useState<IpNetAction>('ban')
  const [loading, setLoading] = useState(false)
  const [initializedFor, setInitializedFor] = useState<number | null>(null)

  if (ipNet && initializedFor !== ipNet.id) {
    setInitializedFor(ipNet.id)
    setAction(ipNet.action)
  }

  const handleSubmit = async () => {
    if (!ipNet) {
      return
    }

    setLoading(true)
    try {
      await ipApi.updateAction(ipNet.id, action)
      onMessage(`成功修改 ${ipNet.ip_net} 的行为为 ${actionLabel(action)}`)
      onSuccess()
      onClose()
    } catch (err) {
      onMessage(errorMessage(err, '修改失败'), 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={Boolean(ipNet)} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>修改IP行为</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          当前IP: {ipNet?.ip_net}
        </Typography>
        <FormControl fullWidth size="small">
          <InputLabel>选择新的行为</InputLabel>
          <Select
            value={action}
            label="选择新的行为"
            onChange={(event) => setAction(event.target.value as IpNetAction)}
          >
            {(['ban', 'allow'] as IpNetAction[]).map((item) => (
              <MenuItem key={item} value={item}>
                {actionLabel(item)}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} color="primary">
          取消
        </Button>
        <Button onClick={() => void handleSubmit()} color="primary" disabled={loading}>
          {loading ? '修改中...' : '修改'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
