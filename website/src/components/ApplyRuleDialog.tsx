import { useEffect, useState } from 'react'
import {
  Box,
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
import { ipApi } from '../api/ip'
import { groupApi } from '../api/group'
import { errorMessage } from '../api/client'
import type { BatchResult, IpGroup, IpNetAction } from '../api/types'

interface ApplyRuleDialogProps {
  open: boolean
  /** 目标 IP 列表（精确 IP，不含 CIDR） */
  ips: string[]
  /** 预选动作 */
  defaultAction: IpNetAction
  onClose: () => void
  onMessage: (message: string, severity?: 'success' | 'error' | 'warning' | 'info') => void
  onSuccess: (result: BatchResult) => void
}

/** 对一批实时流量中的 IP 应用封禁/加白规则（复用导入接口的 upsert 语义） */
export const ApplyRuleDialog = ({
  open,
  ips,
  defaultAction,
  onClose,
  onMessage,
  onSuccess,
}: ApplyRuleDialogProps) => {
  const [action, setAction] = useState<IpNetAction>(defaultAction)
  const [groupId, setGroupId] = useState<number | ''>('')
  const [groups, setGroups] = useState<IpGroup[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open) {
      return
    }
    setAction(defaultAction)
    // 打开时加载组列表并默认选中默认组
    groupApi
      .list()
      .then((list) => {
        setGroups(list)
        setGroupId((list.find((g) => g.is_default) ?? list[0])?.id ?? '')
      })
      .catch(() => setGroups([]))
  }, [open, defaultAction])

  const handleSubmit = async () => {
    if (groupId === '') {
      onMessage('请选择目标组', 'warning')
      return
    }

    setLoading(true)
    try {
      const result = await ipApi.import({ text: ips.join(', '), group_id: groupId, action })
      onSuccess(result)
      onClose()
    } catch (err) {
      onMessage(errorMessage(err, '批量操作失败'), 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog
      open={open}
      onClose={() => {
        if (!loading) onClose()
      }}
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle>应用规则</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          将对选中的 {ips.length} 个 IP 应用规则；已存在规则的 IP 仅更新动作
        </Typography>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <FormControl fullWidth size="small">
            <InputLabel>动作</InputLabel>
            <Select
              value={action}
              label="动作"
              onChange={(event) => setAction(event.target.value as IpNetAction)}
            >
              <MenuItem value="ban">封禁（deny）</MenuItem>
              <MenuItem value="allow">加白（allow）</MenuItem>
            </Select>
          </FormControl>
          <FormControl fullWidth size="small">
            <InputLabel>目标组</InputLabel>
            <Select
              value={groupId}
              label="目标组"
              onChange={(event) => setGroupId(Number(event.target.value))}
            >
              {groups.map((group) => (
                <MenuItem key={group.id} value={group.id}>
                  {group.name}
                  {group.is_default ? '（默认组）' : ''}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          取消
        </Button>
        <Button onClick={() => void handleSubmit()} variant="contained" disabled={loading}>
          {loading ? '应用中...' : '确认应用'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
