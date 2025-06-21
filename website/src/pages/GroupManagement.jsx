import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Button,
  Alert,
  CircularProgress,
  Tooltip,
  TextField,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  IconButton,
  Chip,
  Grid,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  Save as SaveIcon,
} from '@mui/icons-material';
import { useMessageSnackbar, MessageSnackbar } from '../components/MessageSnackbar';
import { useConfirmDialog, ConfirmDialog } from '../components/ConfirmDialog';

// 格式化时间戳
const formatTimestamp = (timestamp) => {
  if (!timestamp) return '-';
  const date = new Date(timestamp);
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
};

function GroupManagement() {
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  
  // 创建组相关状态
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [newGroupName, setNewGroupName] = useState('');
  const [newGroupDescription, setNewGroupDescription] = useState('');
  const [createLoading, setCreateLoading] = useState(false);
  
  // 编辑组相关状态
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState(null);
  const [editGroupName, setEditGroupName] = useState('');
  const [editGroupDescription, setEditGroupDescription] = useState('');
  const [editLoading, setEditLoading] = useState(false);
  
  // 使用消息提示Hook
  const { snackbar, showMessage, hideMessage } = useMessageSnackbar();
  
  // 使用确认对话框Hook
  const { confirmDialog, showConfirm, handleConfirm, handleCancel } = useConfirmDialog();

  // 获取组列表
  const fetchGroups = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch('/api/groups');
      const result = await response.json();
      if (result.code === 200) {
        setGroups(result.data || []);
      } else {
        setError('获取组列表失败: ' + result.message);
      }
    } catch (error) {
      setError('网络请求失败');
      console.error('获取组列表失败:', error);
    } finally {
      setLoading(false);
    }
  };

  // 创建新组
  const createGroup = async () => {
    if (!newGroupName.trim()) {
      showMessage('请输入组名称', 'warning');
      return;
    }

    setCreateLoading(true);
    try {
      const response = await fetch('/api/group', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newGroupName.trim(),
          description: newGroupDescription.trim()
        })
      });
      const result = await response.json();
      if (result.code === 200) {
        showMessage('组创建成功');
        setCreateDialogOpen(false);
        setNewGroupName('');
        setNewGroupDescription('');
        await fetchGroups(); // 刷新列表
      } else {
        showMessage('创建失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('创建失败: 网络错误', 'error');
    } finally {
      setCreateLoading(false);
    }
  };

  // 打开编辑对话框
  const openEditDialog = (group) => {
    setEditingGroup(group);
    setEditGroupName(group.name);
    setEditGroupDescription(group.description || '');
    setEditDialogOpen(true);
  };

  // 保存编辑
  const saveEdit = async () => {
    if (!editGroupName.trim()) {
      showMessage('请输入组名称', 'warning');
      return;
    }

    setEditLoading(true);
    try {
      const response = await fetch('/api/group', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: editingGroup.id,
          name: editGroupName.trim(),
          description: editGroupDescription.trim()
        })
      });
      const result = await response.json();
      if (result.code === 200) {
        showMessage('组更新成功');
        setEditDialogOpen(false);
        setEditingGroup(null);
        await fetchGroups(); // 刷新列表
      } else {
        showMessage('更新失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('更新失败: 网络错误', 'error');
    } finally {
      setEditLoading(false);
    }
  };

  // 删除组
  const deleteGroup = async (group) => {
    showConfirm(
      '确认删除',
      `确定要删除组 "${group.name}" 吗？\n\n注意：删除组后，该组下的所有IP将被归到默认组`,
      'warning',
      async () => {
        try {
          const response = await fetch(`/api/group/${group.id}`, {
            method: 'DELETE'
          });
          const result = await response.json();
          if (result.code === 200) {
            showMessage('组删除成功');
            await fetchGroups(); // 刷新列表
          } else {
            showMessage('删除失败: ' + result.message, 'error');
          }
        } catch (error) {
          showMessage('删除失败: 网络错误', 'error');
        }
      }
    );
  };

  // 处理创建对话框关闭
  const handleCreateDialogClose = () => {
    setCreateDialogOpen(false);
    setNewGroupName('');
    setNewGroupDescription('');
  };

  // 处理编辑对话框关闭
  const handleEditDialogClose = () => {
    setEditDialogOpen(false);
    setEditingGroup(null);
    setEditGroupName('');
    setEditGroupDescription('');
  };

  // 初始化
  useEffect(() => {
    fetchGroups();
  }, []);

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        组管理
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {/* 操作栏 */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <Button
            variant="contained"
            onClick={() => setCreateDialogOpen(true)}
            startIcon={<AddIcon />}
            color="primary"
          >
            创建新组
          </Button>
          <Button
            variant="outlined"
            onClick={fetchGroups}
            startIcon={<RefreshIcon />}
            disabled={loading}
          >
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
          <Table>
            <TableHead>
              <TableRow>
                <TableCell sx={{ fontWeight: 'bold' }}>组名称</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>描述</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>状态</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>创建时间</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>更新时间</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    <CircularProgress size={24} />
                    <Typography sx={{ ml: 1 }}>加载中...</Typography>
                  </TableCell>
                </TableRow>
              ) : groups.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    暂无组，请创建第一个组
                  </TableCell>
                </TableRow>
              ) : (
                groups.map((group) => (
                  <TableRow key={group.id} hover>
                    <TableCell sx={{ fontWeight: 'medium' }}>
                      {group.name}
                    </TableCell>
                    <TableCell>
                      {group.description || (
                        <Typography variant="body2" color="text.secondary">
                          无描述
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      {group.is_default ? (
                        <Chip 
                          label="默认组" 
                          size="small" 
                          color="primary" 
                          variant="filled"
                        />
                      ) : (
                        <Chip 
                          label="普通组" 
                          size="small" 
                          color="default" 
                          variant="outlined"
                        />
                      )}
                    </TableCell>
                    <TableCell sx={{ color: 'text.secondary', fontSize: '0.875rem' }}>
                      {formatTimestamp(group.created_at)}
                    </TableCell>
                    <TableCell sx={{ color: 'text.secondary', fontSize: '0.875rem' }}>
                      {formatTimestamp(group.updated_at)}
                    </TableCell>
                    <TableCell>
                      <Box sx={{ display: 'flex', gap: 1 }}>
                        <Tooltip title="编辑组">
                          <IconButton
                            size="small"
                            color="primary"
                            onClick={() => openEditDialog(group)}
                          >
                            <EditIcon />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title={group.is_default ? "默认组不能被删除" : "删除组"}>
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

      {/* 创建组对话框 */}
      <Dialog open={createDialogOpen} onClose={handleCreateDialogClose} maxWidth="sm" fullWidth>
        <DialogTitle>创建新组</DialogTitle>
        <DialogContent>
          <Grid container spacing={2} sx={{ mt: 1 }}>
            <Grid item xs={12}>
              <TextField
                fullWidth
                label="组名称"
                placeholder="请输入组名称"
                value={newGroupName}
                onChange={(e) => setNewGroupName(e.target.value)}
                disabled={createLoading}
                autoFocus
              />
            </Grid>
            <Grid item xs={12}>
              <TextField
                fullWidth
                label="组描述"
                placeholder="请输入组描述（可选）"
                value={newGroupDescription}
                onChange={(e) => setNewGroupDescription(e.target.value)}
                disabled={createLoading}
                multiline
                rows={3}
              />
            </Grid>
          </Grid>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCreateDialogClose} disabled={createLoading}>
            取消
          </Button>
          <Button 
            onClick={createGroup} 
            variant="contained" 
            disabled={createLoading || !newGroupName.trim()}
            startIcon={createLoading ? <CircularProgress size={16} /> : <SaveIcon />}
          >
            {createLoading ? '创建中...' : '创建'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 编辑组对话框 */}
      <Dialog open={editDialogOpen} onClose={handleEditDialogClose} maxWidth="sm" fullWidth>
        <DialogTitle>编辑组</DialogTitle>
        <DialogContent>
          <Grid container spacing={2} sx={{ mt: 1 }}>
            <Grid item xs={12}>
              <TextField
                fullWidth
                label="组名称"
                placeholder="请输入组名称"
                value={editGroupName}
                onChange={(e) => setEditGroupName(e.target.value)}
                disabled={editLoading}
                autoFocus
              />
            </Grid>
            <Grid item xs={12}>
              <TextField
                fullWidth
                label="组描述"
                placeholder="请输入组描述（可选）"
                value={editGroupDescription}
                onChange={(e) => setEditGroupDescription(e.target.value)}
                disabled={editLoading}
                multiline
                rows={3}
              />
            </Grid>
          </Grid>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleEditDialogClose} disabled={editLoading}>
            取消
          </Button>
          <Button 
            onClick={saveEdit} 
            variant="contained" 
            disabled={editLoading || !editGroupName.trim()}
            startIcon={editLoading ? <CircularProgress size={16} /> : <SaveIcon />}
          >
            {editLoading ? '保存中...' : '保存'}
          </Button>
        </DialogActions>
      </Dialog>

      <MessageSnackbar snackbar={snackbar} onClose={hideMessage} />
      <ConfirmDialog 
        confirmDialog={confirmDialog} 
        onConfirm={handleConfirm} 
        onCancel={handleCancel} 
      />
    </Box>
  );
}

export default GroupManagement; 