import { useEffect, useState } from 'react'
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  Grid,
  InputLabel,
  MenuItem,
  Radio,
  RadioGroup,
  Select,
  TextField,
  Typography,
} from '@mui/material'
import { ipApi } from '../../api/ip'
import { errorMessage } from '../../api/client'
import type { BatchResult, IpGroup, IpNetAction } from '../../api/types'
import { actionLabel } from '../../utils/actions'

interface ImportDialogProps {
  open: boolean
  groups: IpGroup[]
  defaultGroupId: number | null
  onClose: () => void
  onImported: (result: BatchResult) => void
  onError: (message: string) => void
}

type ImportMode = 'text' | 'url'

/** 批量导入 IP/CIDR 规则（文本粘贴或 URL 拉取） */
export const ImportDialog = ({
  open,
  groups,
  defaultGroupId,
  onClose,
  onImported,
  onError,
}: ImportDialogProps) => {
  const [mode, setMode] = useState<ImportMode>('text')
  const [text, setText] = useState('')
  const [url, setUrl] = useState('')
  const [action, setAction] = useState<IpNetAction>('ban')
  const [groupId, setGroupId] = useState<number | ''>('')
  const [loading, setLoading] = useState(false)

  // 打开对话框时同步默认组（组列表可能晚于组件挂载加载完成）
  useEffect(() => {
    if (open && defaultGroupId !== null) {
      setGroupId(defaultGroupId)
    }
  }, [open, defaultGroupId])

  const handleImport = async () => {
    const content = mode === 'text' ? text.trim() : url.trim()
    if (!content) {
      onError('请输入要导入的内容')
      return
    }
    if (groupId === '') {
      onError('请选择要添加到的组')
      return
    }

    setLoading(true)
    try {
      const result = await ipApi.import({
        [mode]: content,
        group_id: groupId,
        action,
      })
      onImported(result)
      setText('')
      setUrl('')
      onClose()
    } catch (err) {
      onError(errorMessage(err, '导入失败'))
    } finally {
      setLoading(false)
    }
  }

  const canSubmit = Boolean((mode === 'text' ? text.trim() : url.trim()) && groupId !== '')

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>导入规则</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          支持单个IP或CIDR网段，通过文本批量粘贴或从URL拉取
        </Typography>

        <Box sx={{ mb: 2 }}>
          <RadioGroup row value={mode} onChange={(event) => setMode(event.target.value as ImportMode)}>
            <FormControlLabel value="text" control={<Radio />} label="文本输入" />
            <FormControlLabel value="url" control={<Radio />} label="URL导入" />
          </RadioGroup>
        </Box>

        {mode === 'text' ? (
          <TextField
            fullWidth
            multiline
            rows={8}
            label="IP地址或CIDR列表"
            placeholder="例如：192.168.1.1, 10.0.0.0/8, 172.16.0.1"
            value={text}
            onChange={(event) => setText(event.target.value)}
            disabled={loading}
            sx={{ mb: 2 }}
          />
        ) : (
          <TextField
            fullWidth
            label="URL地址"
            placeholder="例如：https://example.com/ip-list.txt"
            value={url}
            onChange={(event) => setUrl(event.target.value)}
            disabled={loading}
            sx={{ mb: 2 }}
          />
        )}

        <Grid container spacing={2}>
          <Grid size={{ xs: 12, sm: 6 }}>
            <FormControl fullWidth size="small" disabled={loading}>
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
          </Grid>
          <Grid size={{ xs: 12, sm: 6 }}>
            <FormControl fullWidth size="small" disabled={loading}>
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
          </Grid>
        </Grid>

        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          {mode === 'text'
            ? '支持格式：单个IP（如：192.168.1.1）或CIDR网段（如：192.168.1.0/24），多个IP可用逗号、空格或换行分隔'
            : 'URL 应返回纯文本格式的IP列表，每行一个IP或CIDR'}
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          取消
        </Button>
        <Button onClick={() => void handleImport()} color="primary" disabled={loading || !canSubmit}>
          {loading ? '导入中...' : '确认导入'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
