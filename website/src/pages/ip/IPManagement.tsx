import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  Add as AddIcon,
  Block as BlockIcon,
  Clear as ClearIcon,
  Delete as DeleteIcon,
  Edit as EditIcon,
  Refresh as RefreshIcon,
} from '@mui/icons-material'
import { ipApi } from '../../api/ip'
import { groupApi } from '../../api/group'
import { errorMessage } from '../../api/client'
import type { BatchResult, IpGroup, IpNet, IpNetAction } from '../../api/types'
import { MessageSnackbar } from '../../components/MessageSnackbar'
import { useMessageSnackbar } from '../../hooks/useMessageSnackbar'
import { ConfirmDialog, useConfirmDialog } from '../../components/ConfirmDialog'
import { EmptyState } from '../../components/EmptyState'
import { RowsPerPageControl } from '../../components/RowsPerPageControl'
import { useDebounce } from '../../hooks/useDebounce'
import { formatTimestamp } from '../../utils/format'
import { actionColor, actionLabel } from '../../utils/actions'
import { ImportDialog } from './ImportDialog'
import { ChangeActionDialog, ChangeGroupDialog } from './IpRowDialogs'
import { BatchSetActionDialog, BatchSetGroupDialog } from './BatchDialogs'

const PAGE_SIZES = [10, 25, 50, 100]

function IPManagement() {
  const [searchParams, setSearchParams] = useSearchParams()

  // ---- URL 持久化的列表状态：刷新不丢、可分享 ----
  const readNumberParam = (key: string, fallback: number) => {
    const value = Number(searchParams.get(key))
    return Number.isFinite(value) && value > 0 ? value : fallback
  }
  const page = readNumberParam('page', 1) // URL 中从 1 开始
  const pageSize = readNumberParam('page_size', 25)
  const groupIdFilter = searchParams.get('group') ?? ''
  const actionFilter = (searchParams.get('action') ?? '') as IpNetAction | ''
  const search = searchParams.get('q') ?? ''

  const updateParams = (updates: Record<string, string | number | undefined>) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        for (const [key, value] of Object.entries(updates)) {
          if (value === undefined || value === '' || value === null) {
            next.delete(key)
          } else {
            next.set(key, String(value))
          }
        }
        return next
      },
      { replace: true },
    )
  }

  // 搜索输入本地即时回显，防抖后写入 URL 并触发查询
  const [searchInput, setSearchInput] = useState(search)
  const debouncedSearch = useDebounce(searchInput, 300)
  useEffect(() => {
    if (debouncedSearch !== search) {
      updateParams({ q: debouncedSearch || undefined, page: undefined })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearch])

  // ---- 数据状态 ----
  const [items, setItems] = useState<IpNet[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [initialLoading, setInitialLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [groups, setGroups] = useState<IpGroup[]>([])

  // ---- 选择与对话框状态 ----
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [importOpen, setImportOpen] = useState(false)
  const [batchActionOpen, setBatchActionOpen] = useState(false)
  const [batchGroupOpen, setBatchGroupOpen] = useState(false)
  const [changeGroupTarget, setChangeGroupTarget] = useState<IpNet | null>(null)
  const [changeActionTarget, setChangeActionTarget] = useState<IpNet | null>(null)

  const { snackbar, showMessage, hideMessage } = useMessageSnackbar()
  const { showConfirm, confirmState, handleConfirm, handleCancel } = useConfirmDialog()

  const fetchGroups = useCallback(async () => {
    try {
      const data = await groupApi.list()
      setGroups(data)
    } catch (err) {
      console.error('获取组列表失败:', err)
    }
  }, [])

  const fetchIpNets = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const result = await ipApi.list({
        page,
        page_size: pageSize,
        group_id: groupIdFilter ? Number(groupIdFilter) : undefined,
        action: actionFilter || undefined,
        search: search || undefined,
      })
      setItems(result.items)
      setTotal(result.total)
    } catch (err) {
      setError(errorMessage(err, '获取IP列表失败'))
    } finally {
      setLoading(false)
      setInitialLoading(false)
    }
  }, [page, pageSize, groupIdFilter, actionFilter, search])

  useEffect(() => {
    void fetchGroups()
  }, [fetchGroups])

  useEffect(() => {
    void fetchIpNets()
  }, [fetchIpNets])

  // ---- 列表操作 ----
  const refresh = useCallback(async () => {
    await fetchIpNets()
    await fetchGroups()
  }, [fetchIpNets, fetchGroups])

  const deleteIp = (ipNet: IpNet) => {
    showConfirm(
      '确认删除',
      `确定要删除 ${ipNet.ip_net} 的规则吗？\n\n删除后将解除对应的防火墙规则，此操作不可撤销。`,
      'error',
      () => {
        void (async () => {
          try {
            await ipApi.remove(ipNet.id)
            showMessage(`成功删除 ${ipNet.ip_net}`)
            await refresh()
          } catch (err) {
            showMessage(errorMessage(err, '删除失败'), 'error')
          }
        })()
      },
    )
  }

  // ---- 选择 ----
  const currentPageIds = useMemo(() => items.map((item) => item.id), [items])
  const selectedOnPage = useMemo(
    () => currentPageIds.filter((id) => selectedIds.has(id)),
    [currentPageIds, selectedIds],
  )
  const allOnPageSelected = currentPageIds.length > 0 && selectedOnPage.length === currentPageIds.length

  const toggleSelect = (id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const toggleSelectAllOnPage = () => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (allOnPageSelected) {
        currentPageIds.forEach((id) => next.delete(id))
      } else {
        currentPageIds.forEach((id) => next.add(id))
      }
      return next
    })
  }

  const clearSelection = () => {
    setSelectedIds(new Set())
  }

  // ---- 批量操作结果提示 ----
  const notifyBatchResult = (operation: string, result: BatchResult) => {
    clearSelection()
    if (result.failed_count === 0) {
      showMessage(`成功${operation} ${result.success_count} 个IP`)
    } else {
      showMessage(
        `${operation}完成：成功 ${result.success_count} 个，失败 ${result.failed_count} 个`,
        'warning',
      )
    }
    void refresh()
  }

  // ---- 分页/筛选切换 ----
  const handleTabChange = (_: React.SyntheticEvent, value: string) => {
    updateParams({ group: value || undefined, page: undefined })
    clearSelection()
  }

  const currentGroup = groups.find((group) => String(group.id) === groupIdFilter)
  const defaultGroupId = groups[0]?.id ?? null
  const selectedIdsList = useMemo(() => Array.from(selectedIds), [selectedIds])

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        IP 管理
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <Paper sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
          <Button
            variant="contained"
            onClick={() => setImportOpen(true)}
            startIcon={<AddIcon />}
            color="primary"
          >
            导入规则
          </Button>
          <Typography variant="body2" color="text.secondary">
            支持IP地址、CIDR网段通过文本批量导入或URL导入
          </Typography>
        </Box>
      </Paper>

      {/* 操作栏 */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
            <Button
              variant="outlined"
              onClick={() => void refresh()}
              startIcon={<RefreshIcon />}
              disabled={loading}
            >
              刷新列表
            </Button>
            <TextField
              size="small"
              label="搜索地址"
              placeholder="例如：192.168"
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              sx={{ width: 220 }}
            />
            <Typography variant="body2" color="text.secondary">
              共 {total.toLocaleString()} 个IP
              {currentGroup && <span>（组：{currentGroup.name}）</span>}
              {(actionFilter || search) && <span>（已过滤）</span>}
            </Typography>
          </Box>

          {/* 行为过滤 */}
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
            <FormControl size="small" sx={{ minWidth: 140 }}>
              <InputLabel>行为过滤</InputLabel>
              <Select
                value={actionFilter}
                label="行为过滤"
                onChange={(event) =>
                  updateParams({ action: (event.target.value as string) || undefined, page: undefined })
                }
              >
                <MenuItem value="">全部</MenuItem>
                <MenuItem value="ban">禁用</MenuItem>
                <MenuItem value="allow">允许</MenuItem>
              </Select>
            </FormControl>
            {(actionFilter || search) && (
              <Button
                variant="outlined"
                size="small"
                onClick={() => {
                  setSearchInput('')
                  updateParams({ action: undefined, q: undefined, page: undefined })
                }}
              >
                清除过滤
              </Button>
            )}
          </Box>

          {/* 批量操作栏 */}
          {selectedIds.size > 0 && (
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 2,
                flexWrap: 'wrap',
                pt: 1,
                borderTop: 1,
                borderColor: 'divider',
              }}
            >
              <Typography variant="subtitle2" color="primary">
                批量操作：
              </Typography>
              <Button
                variant="outlined"
                color="error"
                startIcon={<DeleteIcon />}
                onClick={() =>
                  showConfirm(
                    '确认批量删除',
                    `确定要删除选中的 ${selectedIds.size} 个IP吗？\n\n此操作不可撤销，请谨慎操作。`,
                    'error',
                    () => {
                      void (async () => {
                        try {
                          const result = await ipApi.batchDelete(selectedIdsList)
                          notifyBatchResult('删除', result)
                        } catch (err) {
                          showMessage(errorMessage(err, '批量删除失败'), 'error')
                        }
                      })()
                    },
                  )
                }
              >
                批量删除 ({selectedIds.size})
              </Button>
              <Button
                variant="outlined"
                color="info"
                startIcon={<EditIcon />}
                onClick={() => setBatchActionOpen(true)}
              >
                批量设置行为 ({selectedIds.size})
              </Button>
              <Button
                variant="outlined"
                color="secondary"
                startIcon={<EditIcon />}
                onClick={() => setBatchGroupOpen(true)}
              >
                批量设置组 ({selectedIds.size})
              </Button>
              <Button variant="outlined" color="inherit" startIcon={<ClearIcon />} onClick={clearSelection}>
                清除选择
              </Button>
            </Box>
          )}
        </Box>
      </Paper>

      {/* 组过滤标签页（徽标数量来自服务端） */}
      <Paper sx={{ mb: 2 }}>
        <Tabs
          value={groupIdFilter}
          onChange={handleTabChange}
          variant="scrollable"
          scrollButtons="auto"
          sx={{ borderBottom: 1, borderColor: 'divider' }}
        >
          <Tab
            value=""
            label={
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                全部
                <Chip size="small" color="primary" variant="outlined" label={groups.reduce((sum, group) => sum + group.ip_count, 0)} />
              </Box>
            }
          />
          {groups.map((group) => (
            <Tab
              key={group.id}
              value={String(group.id)}
              label={
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  {group.name}
                  <Chip size="small" color="primary" variant="outlined" label={group.ip_count} />
                </Box>
              }
            />
          ))}
        </Tabs>
      </Paper>

      {/* IP列表表格 */}
      <Paper>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell padding="checkbox">
                  <Checkbox
                    indeterminate={selectedOnPage.length > 0 && !allOnPageSelected}
                    checked={allOnPageSelected}
                    onChange={toggleSelectAllOnPage}
                    disabled={loading || currentPageIds.length === 0}
                  />
                </TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>IP地址或CIDR</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>所属组</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>行为</TableCell>
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
              {loading && initialLoading ? (
                <TableRow>
                  <TableCell colSpan={7} align="center">
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 3 }}>
                      <CircularProgress size={24} />
                      <Typography sx={{ ml: 1 }}>加载中...</Typography>
                    </Box>
                  </TableCell>
                </TableRow>
              ) : items.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7}>
                    <EmptyState
                      icon={<BlockIcon />}
                      title={currentGroup ? '该组暂无IP规则' : '暂无IP规则'}
                      description="导入规则后即可对指定IP/CIDR执行禁用或放行"
                      actionLabel="导入第一条规则"
                      onAction={() => setImportOpen(true)}
                    />
                  </TableCell>
                </TableRow>
              ) : (
                items.map((ipNet) => (
                  <TableRow key={ipNet.id} hover>
                    <TableCell padding="checkbox">
                      <Checkbox
                        checked={selectedIds.has(ipNet.id)}
                        onChange={() => toggleSelect(ipNet.id)}
                      />
                    </TableCell>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: '1rem' }}>
                      {ipNet.ip_net}
                    </TableCell>
                    <TableCell>
                      {ipNet.group ? (
                        <Chip label={ipNet.group.name} size="small" color="primary" variant="outlined" />
                      ) : (
                        <Typography variant="body2" color="text.secondary">
                          未分组
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={actionLabel(ipNet.action)}
                        size="small"
                        color={actionColor(ipNet.action)}
                      />
                    </TableCell>
                    <TableCell
                      sx={{ color: 'text.secondary', fontSize: '0.875rem', display: { xs: 'none', md: 'table-cell' } }}
                    >
                      {formatTimestamp(ipNet.created_at)}
                    </TableCell>
                    <TableCell
                      sx={{ color: 'text.secondary', fontSize: '0.875rem', display: { xs: 'none', lg: 'table-cell' } }}
                    >
                      {formatTimestamp(ipNet.updated_at)}
                    </TableCell>
                    <TableCell>
                      <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                        <Tooltip title="删除此IP或CIDR">
                          <Button
                            variant="outlined"
                            size="small"
                            color="error"
                            onClick={() => deleteIp(ipNet)}
                          >
                            删除
                          </Button>
                        </Tooltip>
                        <Tooltip title="修改所属组">
                          <Button
                            variant="outlined"
                            size="small"
                            color="secondary"
                            onClick={() => setChangeGroupTarget(ipNet)}
                          >
                            修改组
                          </Button>
                        </Tooltip>
                        <Tooltip title="修改行为">
                          <Button
                            variant="outlined"
                            size="small"
                            color="info"
                            onClick={() => setChangeActionTarget(ipNet)}
                          >
                            修改行为
                          </Button>
                        </Tooltip>
                      </Box>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>

        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: 2,
            p: 2,
          }}
        >
          <RowsPerPageControl
            value={pageSize}
            onChange={(value) => updateParams({ page_size: value, page: undefined })}
            options={PAGE_SIZES}
          />
          <TablePagination
            component="div"
            count={total}
            page={page - 1}
            onPageChange={(_, newPage) => updateParams({ page: newPage + 1 })}
            rowsPerPage={pageSize}
            onRowsPerPageChange={() => {
              // 每页条数由上方 RowsPerPageControl 控制
            }}
            rowsPerPageOptions={[pageSize]}
            labelRowsPerPage="每页显示:"
            labelDisplayedRows={({ from, to, count }) => `${from}-${to} / ${count}`}
            showFirstButton
            showLastButton
          />
        </Box>
      </Paper>

      {/* 对话框 */}
      <ImportDialog
        open={importOpen}
        groups={groups}
        defaultGroupId={groupIdFilter ? Number(groupIdFilter) : defaultGroupId}
        onClose={() => setImportOpen(false)}
        onImported={(result) => {
          if (result.success_count > 0) {
            showMessage(
              `成功导入 ${result.success_count} 个IP${result.failed_count > 0 ? `，失败 ${result.failed_count} 个` : ''}`,
              result.failed_count > 0 ? 'warning' : 'success',
            )
          } else {
            showMessage('没有成功导入任何IP', 'warning')
          }
          void refresh()
        }}
        onError={(message) => showMessage(message, 'error')}
      />

      <ChangeGroupDialog
        ipNet={changeGroupTarget}
        groups={groups}
        onClose={() => setChangeGroupTarget(null)}
        onMessage={showMessage}
        onSuccess={() => void refresh()}
      />

      <ChangeActionDialog
        ipNet={changeActionTarget}
        onClose={() => setChangeActionTarget(null)}
        onMessage={showMessage}
        onSuccess={() => void refresh()}
      />

      <BatchSetActionDialog
        open={batchActionOpen}
        ids={selectedIdsList}
        onClose={() => setBatchActionOpen(false)}
        onMessage={showMessage}
        onSuccess={(result) => notifyBatchResult('设置行为', result)}
      />

      <BatchSetGroupDialog
        open={batchGroupOpen}
        ids={selectedIdsList}
        groups={groups}
        onClose={() => setBatchGroupOpen(false)}
        onMessage={showMessage}
        onSuccess={(result) => notifyBatchResult('设置组', result)}
      />

      <MessageSnackbar snackbar={snackbar} onClose={hideMessage} />
      <ConfirmDialog confirmState={confirmState} onConfirm={handleConfirm} onCancel={handleCancel} />
    </Box>
  )
}

export default IPManagement
