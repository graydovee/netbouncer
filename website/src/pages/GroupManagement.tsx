import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  Edit as EditIcon,
  Group as GroupIcon,
  Refresh as RefreshIcon,
  Save as SaveIcon,
} from '@mui/icons-material'
import { groupApi } from '../api/group'
import { errorMessage } from '../api/client'
import type { IpGroup } from '../api/types'
import { MessageSnackbar } from '../components/MessageSnackbar'
import { useMessageSnackbar } from '../hooks/useMessageSnackbar'
import { ConfirmDialog, useConfirmDialog } from '../components/ConfirmDialog'
import { EmptyState } from '../components/EmptyState'
import { formatTimestamp } from '../utils/format'

interface GroupFormDialogProps {
  open: boolean
  title: string
  initialName: string
  initialDescription: string
  loading: boolean
  submitLabel: string
  onClose: () => void
  onSubmit: (name: string, description: string) => void
}

/** 创建/编辑组共用表单对话框 */
function GroupFormDialog({
  open,
  title,
  initialName,
  initialDescription,
  loading,
  submitLabel,
  onClose,
  onSubmit,
}: GroupFormDialogProps) {
  const [name, setName] = useState(initialName)
  const [description, setDescription] = useState(initialDescription)

  // 打开时同步初始值
  useEffect(() => {
    if (open) {
      setName(initialName)
      setDescription(initialDescription)
    }
  }, [open, initialName, initialDescription])

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, mt: 1 }}>
          <TextField
            fullWidth
            label="组名称"
            placeholder="请输入组名称"
            value={name}
            onChange={(event) => setName(event.target.value)}
            disabled={loading}
            autoFocus
          />
          <TextField
            fullWidth
            label="组描述"
            placeholder="请输入组描述（可选）"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            disabled={loading}
            multiline
            rows={3}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          取消
        </Button>
        <Button
          onClick={() => onSubmit(name.trim(), description.trim())}
          variant="contained"
          disabled={loading || !name.trim()}
          startIcon={loading ? <CircularProgress size={16} /> : <SaveIcon />}
        >
          {loading ? '保存中...' : submitLabel}
        </Button>
      </DialogActions>
    </Dialog>
  )
}

function GroupManagement() {
  const [groups, setGroups] = useState<IpGroup[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [createOpen, setCreateOpen] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)

  const [editingGroup, setEditingGroup] = useState<IpGroup | null>(null)
  const [editLoading, setEditLoading] = useState(false)

  const { snackbar, showMessage, hideMessage } = useMessageSnackbar()
  const { showConfirm, confirmState, handleConfirm, handleCancel } = useConfirmDialog()

  const fetchGroups = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setGroups(await groupApi.list())
    } catch (err) {
      setError(errorMessage(err, '获取组列表失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchGroups()
  }, [fetchGroups])

  const createGroup = async (name: string, description: string) => {
    setCreateLoading(true)
    try {
      await groupApi.create({ name, description })
      showMessage('组创建成功')
      setCreateOpen(false)
      await fetchGroups()
    } catch (err) {
      showMessage(errorMessage(err, '创建失败'), 'error')
    } finally {
      setCreateLoading(false)
    }
  }

  const saveEdit = async (name: string, description: string) => {
    if (!editingGroup) {
      return
    }

    setEditLoading(true)
    try {
      await groupApi.update(editingGroup.id, { name, description })
      showMessage('组更新成功')
      setEditingGroup(null)
      await fetchGroups()
    } catch (err) {
      showMessage(errorMessage(err, '更新失败'), 'error')
    } finally {
      setEditLoading(false)
    }
  }

  const deleteGroup = (group: IpGroup) => {
    showConfirm(
      '确认删除',
      `确定要删除组 "${group.name}" 吗？\n\n注意：删除组后，该组下的所有IP将被归到默认组。`,
      'warning',
      () => {
        void (async () => {
          try {
            await groupApi.remove(group.id)
            showMessage('组删除成功')
            await fetchGroups()
          } catch (err) {
            showMessage(errorMessage(err, '删除失败'), 'error')
          }
        })()
      },
    )
  }

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        组管理
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* 操作栏 */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
          <Button
            variant="contained"
            onClick={() => setCreateOpen(true)}
            startIcon={<AddIcon />}
            color="primary"
          >
            创建新组
          </Button>
          <Button variant="outlined" onClick={() => void fetchGroups()} startIcon={<RefreshIcon />} disabled={loading}>
            刷新列表
          </Button>
          <Typography variant="body2" color="text.secondary">
            共 {groups.length} 个组
          </Typography>
        </Box>
      </Paper>

      {/* 组列表表格 */}
      <Paper>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell sx={{ fontWeight: 'bold' }}>组名称</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>描述</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>状态</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>IP数量</TableCell>
                <TableCell sx={{ fontWeight: 'bold', display: { xs: 'none', md: 'table-cell' } }}>
                  创建时间
                </TableCell>
                <TableCell sx={{ fontWeight: 'bold', display: { xs: 'none', lg: 'table-cell' } }}>
                  更新时间
                </TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading && groups.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} align="center">
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 3 }}>
                      <CircularProgress size={24} />
                      <Typography sx={{ ml: 1 }}>加载中...</Typography>
                    </Box>
                  </TableCell>
                </TableRow>
              ) : groups.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7}>
                    <EmptyState
                      icon={<GroupIcon />}
                      title="暂无组"
                      description="创建组后可以按组管理IP规则"
                      actionLabel="创建第一个组"
                      onAction={() => setCreateOpen(true)}
                    />
                  </TableCell>
                </TableRow>
              ) : (
                groups.map((group) => (
                  <TableRow key={group.id} hover>
                    <TableCell sx={{ fontWeight: 500 }}>{group.name}</TableCell>
                    <TableCell>
                      {group.description || (
                        <Typography variant="body2" color="text.secondary">
                          无描述
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      {group.is_default ? (
                        <Chip label="默认组" size="small" color="primary" />
                      ) : (
                        <Chip label="普通组" size="small" color="default" variant="outlined" />
                      )}
                    </TableCell>
                    <TableCell>{group.ip_count.toLocaleString()}</TableCell>
                    <TableCell
                      sx={{ color: 'text.secondary', fontSize: '0.875rem', display: { xs: 'none', md: 'table-cell' } }}
                    >
                      {formatTimestamp(group.created_at)}
                    </TableCell>
                    <TableCell
                      sx={{ color: 'text.secondary', fontSize: '0.875rem', display: { xs: 'none', lg: 'table-cell' } }}
                    >
                      {formatTimestamp(group.updated_at)}
                    </TableCell>
                    <TableCell>
                      <Box sx={{ display: 'flex', gap: 0.5 }}>
                        <Tooltip title="编辑组">
                          <IconButton size="small" color="primary" onClick={() => setEditingGroup(group)}>
                            <EditIcon />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title={group.is_default ? '默认组不能被删除' : '删除组'}>
                          <span>
                            <IconButton
                              size="small"
                              color="error"
                              onClick={() => deleteGroup(group)}
                              disabled={group.is_default}
                            >
                              <DeleteIcon />
                            </IconButton>
                          </span>
                        </Tooltip>
                      </Box>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      {/* 创建组 */}
      <GroupFormDialog
        open={createOpen}
        title="创建新组"
        initialName=""
        initialDescription=""
        loading={createLoading}
        submitLabel="创建"
        onClose={() => setCreateOpen(false)}
        onSubmit={(name, description) => void createGroup(name, description)}
      />

      {/* 编辑组 */}
      <GroupFormDialog
        open={Boolean(editingGroup)}
        title="编辑组"
        initialName={editingGroup?.name ?? ''}
        initialDescription={editingGroup?.description ?? ''}
        loading={editLoading}
        submitLabel="保存"
        onClose={() => setEditingGroup(null)}
        onSubmit={(name, description) => void saveEdit(name, description)}
      />

      <MessageSnackbar snackbar={snackbar} onClose={hideMessage} />
      <ConfirmDialog confirmState={confirmState} onConfirm={handleConfirm} onCancel={handleCancel} />
    </Box>
  )
}

export default GroupManagement
