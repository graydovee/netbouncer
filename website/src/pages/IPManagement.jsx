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
  Grid,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Chip,
  Tabs,
  Tab,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  Block as BlockIcon,
  Add as AddIcon,
  FilterList as FilterIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
} from '@mui/icons-material';
import { useMessageSnackbar, MessageSnackbar } from '../components/MessageSnackbar';

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

function IPManagement() {
  const [ipNets, setIpNets] = useState([]);
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [groupsLoading, setGroupsLoading] = useState(false);
  const [error, setError] = useState(null);
  const [newIP, setNewIP] = useState('');
  const [selectedGroupId, setSelectedGroupId] = useState('');
  const [selectedAction, setSelectedAction] = useState('ban');
  const [createLoading, setCreateLoading] = useState(false);
  const [selectedTab, setSelectedTab] = useState(0); // 0: 全部, 1+: 按组过滤
  const [editGroupId, setEditGroupId] = useState('');
  const [editGroupName, setEditGroupName] = useState('');
  const [editGroupDescription, setEditGroupDescription] = useState('');
  const [editGroupLoading, setEditGroupLoading] = useState(false);
  const [changeGroupDialog, setChangeGroupDialog] = useState(false);
  const [selectedIP, setSelectedIP] = useState('');
  const [selectedIPData, setSelectedIPData] = useState(null);
  const [newGroupId, setNewGroupId] = useState('');
  const [changeGroupLoading, setChangeGroupLoading] = useState(false);
  const [changeActionDialog, setChangeActionDialog] = useState(false);
  const [newAction, setNewAction] = useState('');
  const [changeActionLoading, setChangeActionLoading] = useState(false);
  const [availableActions, setAvailableActions] = useState([]);
  
  // 使用消息提示Hook
  const { snackbar, showMessage, hideMessage } = useMessageSnackbar();

  // 获取组列表
  const fetchGroups = async () => {
    setGroupsLoading(true);
    try {
      const response = await fetch('/api/group');
      const result = await response.json();
      if (result.code === 200) {
        setGroups(result.data || []);
        // 如果有组，设置第一个为默认选择
        if (result.data && result.data.length > 0 && !selectedGroupId) {
          setSelectedGroupId(result.data[0].id);
        }
      } else {
        console.error('获取组列表失败:', result.message);
      }
    } catch (error) {
      console.error('获取组列表失败:', error);
    } finally {
      setGroupsLoading(false);
    }
  };

  // 获取可用操作列表
  const fetchAvailableActions = async () => {
    try {
      const response = await fetch('/api/ip/action');
      const result = await response.json();
      if (result.code === 200) {
        setAvailableActions(result.data || []);
      } else {
        console.error('获取操作列表失败:', result.message);
      }
    } catch (error) {
      console.error('获取操作列表失败:', error);
    }
  };

  // 获取IP列表
  const fetchIpNets = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch('/api/ip');
      const result = await response.json();
      if (result.code === 200) {
        const ipNetList = result.data || [];
        setIpNets(ipNetList);
      } else {
        setError('获取数据失败: ' + result.message);
      }
    } catch (error) {
      setError('网络请求失败');
      console.error('获取IP列表失败:', error);
    } finally {
      setLoading(false);
    }
  };

  // 根据组ID获取IP列表
  const fetchIpNetsByGroup = async (groupId) => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(`/api/ip/${groupId}`);
      const result = await response.json();
      if (result.code === 200) {
        const ipNetList = result.data || [];
        setIpNets(ipNetList);
      } else {
        setError('获取数据失败: ' + result.message);
      }
    } catch (error) {
      setError('网络请求失败');
      console.error('获取IP列表失败:', error);
    } finally {
      setLoading(false);
    }
  };

  // 验证IP或CIDR格式
  const validateIPOrCIDR = (input) => {
    // 简单的IP或CIDR验证
    const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$/;
    const cidrRegex = /^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/;
    
    if (ipRegex.test(input)) {
      // 验证IP地址的每个段是否在0-255范围内
      const parts = input.split('.');
      return parts.every(part => parseInt(part) >= 0 && parseInt(part) <= 255);
    } else if (cidrRegex.test(input)) {
      // 验证CIDR格式
      const [ip, prefix] = input.split('/');
      const prefixNum = parseInt(prefix);
      if (prefixNum < 0 || prefixNum > 32) return false;
      
      const parts = ip.split('.');
      return parts.every(part => parseInt(part) >= 0 && parseInt(part) <= 255);
    }
    return false;
  };

  // 创建IP或CIDR
  const createIPOrCIDR = async () => {
    if (!newIP.trim()) {
      showMessage('请输入IP地址或CIDR', 'warning');
      return;
    }

    if (!validateIPOrCIDR(newIP.trim())) {
      showMessage('请输入有效的IP地址或CIDR格式（例如：192.168.1.1 或 192.168.1.0/24）', 'error');
      return;
    }

    if (!selectedGroupId) {
      showMessage('请选择要添加到的组', 'warning');
      return;
    }

    setCreateLoading(true);
    try {
      const response = await fetch('/api/ip', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          ip_net: newIP.trim(),
          group_id: parseInt(selectedGroupId),
          action: selectedAction
        })
      });
      const result = await response.json();
      if (result.code === 200) {
        // 创建成功后重新获取列表
        if (selectedTab === 0) {
          await fetchIpNets();
        } else {
          await fetchIpNetsByGroup(selectedGroupId);
        }
        setNewIP(''); // 清空输入框
        showMessage(`成功创建 ${newIP.trim()}`);
      } else {
        showMessage('创建失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('创建失败: 网络错误', 'error');
    } finally {
      setCreateLoading(false);
    }
  };

  // 删除IP
  const deleteIP = async (ipId) => {
    try {
      const response = await fetch(`/api/ip/${ipId}`, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' }
      });
      const result = await response.json();
      if (result.code === 200) {
        // 从列表中移除该IP
        setIpNets(prev => prev.filter(ip => ip.id !== ipId));
        showMessage(`成功删除IP`);
      } else {
        showMessage('删除失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('删除失败: 网络错误', 'error');
    }
  };

  // 处理回车键
  const handleKeyDown = (event) => {
    if (event.key === 'Enter') {
      createIPOrCIDR();
    }
  };

  // 处理标签页切换
  const handleTabChange = (event, newValue) => {
    setSelectedTab(newValue);
    if (newValue === 0) {
      // 显示全部
      fetchIpNets();
    } else {
      // 显示指定组
      const groupId = groups[newValue - 1]?.id;
      if (groupId) {
        fetchIpNetsByGroup(groupId);
      }
    }
  };

  // 初始化
  useEffect(() => {
    fetchGroups();
    fetchIpNets();
    fetchAvailableActions();
  }, []);

  // 构建标签页
  const tabLabels = ['全部', ...groups.map(group => group.name)];

  // 打开编辑组对话框
  const openEditGroupDialog = (groupId, groupName, groupDescription) => {
    setEditGroupId(groupId);
    setEditGroupName(groupName);
    setEditGroupDescription(groupDescription);
  };

  // 保存组信息
  const saveGroup = async () => {
    setEditGroupLoading(true);
    try {
      const response = await fetch('/api/group', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          id: editGroupId,
          name: editGroupName,
          description: editGroupDescription
        })
      });
      const result = await response.json();
      if (result.code === 200) {
        // 更新组列表
        setGroups(prevGroups => prevGroups.map(group =>
          group.id === editGroupId ? { ...group, name: editGroupName, description: editGroupDescription } : group
        ));
        // 如果当前选择的组是编辑的组，重新获取列表
        if (selectedGroupId === editGroupId) {
          await fetchIpNetsByGroup(editGroupId);
        }
        setEditGroupId(''); // 关闭对话框
        showMessage('组信息保存成功');
      } else {
        showMessage('组信息保存失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('组信息保存失败: 网络错误', 'error');
    } finally {
      setEditGroupLoading(false);
    }
  };

  // 打开修改IP所属组对话框
  const openChangeGroupDialog = (ipData) => {
    setSelectedIP(ipData.ip_net);
    setSelectedIPData(ipData);
    setNewGroupId(ipData.group?.id || '');
    setChangeGroupDialog(true);
  };

  // 修改IP所属组
  const changeIPGroup = async () => {
    if (!newGroupId) {
      showMessage('请选择新的组', 'warning');
      return;
    }

    setChangeGroupLoading(true);
    try {
      const response = await fetch('/api/ip/group', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          id: selectedIPData.id,
          group_id: parseInt(newGroupId)
        })
      });
      const result = await response.json();
      if (result.code === 200) {
        // 重新获取列表
        if (selectedTab === 0) {
          await fetchIpNets();
        } else {
          await fetchIpNetsByGroup(selectedGroupId);
        }
        setChangeGroupDialog(false);
        showMessage(`成功修改 ${selectedIP} 的所属组`);
      } else {
        showMessage('修改失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('修改失败: 网络错误', 'error');
    } finally {
      setChangeGroupLoading(false);
    }
  };

  // 打开修改IP行为对话框
  const openChangeActionDialog = (ipData) => {
    setSelectedIP(ipData.ip_net);
    setSelectedIPData(ipData);
    setNewAction(ipData.action);
    setChangeActionDialog(true);
  };

  // 修改IP行为
  const changeIPAction = async () => {
    if (!newAction) {
      showMessage('请选择新的行为', 'warning');
      return;
    }

    setChangeActionLoading(true);
    try {
      const response = await fetch('/api/ip/action', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          id: selectedIPData.id,
          action: newAction
        })
      });
      const result = await response.json();
      if (result.code === 200) {
        // 重新获取列表
        if (selectedTab === 0) {
          await fetchIpNets();
        } else {
          await fetchIpNetsByGroup(selectedGroupId);
        }
        setChangeActionDialog(false);
        showMessage(`成功修改 ${selectedIP} 的行为为 ${newAction}`);
      } else {
        showMessage('修改失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('修改失败: 网络错误', 'error');
    } finally {
      setChangeActionLoading(false);
    }
  };

  // 获取行为显示文本
  const getActionDisplayText = (action) => {
    switch (action) {
      case 'ban':
        return '禁用';
      case 'allow':
        return '允许';
      default:
        return action;
    }
  };

  // 获取行为颜色
  const getActionColor = (action) => {
    switch (action) {
      case 'ban':
        return 'error';
      case 'allow':
        return 'success';
      default:
        return 'default';
    }
  };

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        IP管理
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {/* 创建IP操作栏 */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Typography variant="h6" gutterBottom>
          创建IP或CIDR
        </Typography>
        <Grid container spacing={2} alignItems="center">
          <Grid item xs={12} sm={3} md={2}>
            <TextField
              fullWidth
              label="IP地址或CIDR"
              placeholder="例如10.0.0.1或10.0.0.0/8"
              value={newIP}
              onChange={(e) => setNewIP(e.target.value)}
              onKeyDown={handleKeyDown}
              size="small"
              disabled={createLoading}
            />
          </Grid>
          <Grid item xs={12} sm={3} md={2}>
            <FormControl fullWidth size="small" disabled={createLoading || groupsLoading}>
              <InputLabel>选择组</InputLabel>
              <Select
                value={selectedGroupId}
                label="选择组"
                onChange={(e) => setSelectedGroupId(e.target.value)}
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
          <Grid item xs={12} sm={3} md={2}>
            <FormControl fullWidth size="small" disabled={createLoading}>
              <InputLabel>选择行为</InputLabel>
              <Select
                value={selectedAction}
                label="选择行为"
                onChange={(e) => setSelectedAction(e.target.value)}
              >
                {availableActions.map((action) => (
                  <MenuItem key={action} value={action}>
                    {getActionDisplayText(action)}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>
          <Grid item>
            <Button
              variant="contained"
              onClick={createIPOrCIDR}
              startIcon={<AddIcon />}
              disabled={createLoading || !newIP.trim() || !selectedGroupId}
              color="primary"
            >
              {createLoading ? '创建中...' : '创建'}
            </Button>
          </Grid>
        </Grid>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
          支持单个IP地址（如：192.168.1.1）或CIDR网段（如：192.168.1.0/24）
        </Typography>
      </Paper>

      {/* 组过滤标签页 */}
      <Paper sx={{ mb: 2 }}>
        <Tabs 
          value={selectedTab} 
          onChange={handleTabChange}
          variant="scrollable"
          scrollButtons="auto"
          sx={{ borderBottom: 1, borderColor: 'divider' }}
        >
          {tabLabels.map((label, index) => (
            <Tab 
              key={index} 
              label={
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  {label}
                  {index > 0 && (
                    <Chip 
                      label={ipNets.filter(ip => ip.group?.id === groups[index - 1]?.id).length}
                      size="small"
                      color="primary"
                      variant="outlined"
                    />
                  )}
                </Box>
              }
            />
          ))}
        </Tabs>
      </Paper>

      {/* 操作栏 */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <Button
            variant="outlined"
            onClick={() => {
              if (selectedTab === 0) {
                fetchIpNets();
              } else {
                const groupId = groups[selectedTab - 1]?.id;
                if (groupId) {
                  fetchIpNetsByGroup(groupId);
                }
              }
            }}
            startIcon={<RefreshIcon />}
            disabled={loading}
          >
            刷新列表
          </Button>
          <Typography variant="body2" color="text.secondary">
            共 {ipNets.length} 个IP
            {selectedTab > 0 && groups[selectedTab - 1] && (
              <span>（组：{groups[selectedTab - 1].name}）</span>
            )}
          </Typography>
        </Box>
      </Paper>

      {/* IP列表表格 */}
      <Paper>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell sx={{ fontWeight: 'bold' }}>IP地址或CIDR</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>所属组</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>行为</TableCell>
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
              ) : ipNets.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    {selectedTab === 0 ? '暂无IP记录' : '该组暂无IP记录'}
                  </TableCell>
                </TableRow>
              ) : (
                ipNets.map((ipNet, index) => (
                  <TableRow key={index} hover>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: '1rem' }}>
                      {ipNet.ip_net}
                    </TableCell>
                    <TableCell>
                      {ipNet.group ? (
                        <Chip 
                          label={ipNet.group.name}
                          size="small"
                          color="primary"
                          variant="outlined"
                        />
                      ) : (
                        <Typography variant="body2" color="text.secondary">
                          未分组
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Chip 
                        label={getActionDisplayText(ipNet.action)}
                        size="small"
                        color={getActionColor(ipNet.action)}
                        variant="filled"
                      />
                    </TableCell>
                    <TableCell sx={{ color: 'text.secondary', fontSize: '0.875rem' }}>
                      {formatTimestamp(ipNet.created_at)}
                    </TableCell>
                    <TableCell sx={{ color: 'text.secondary', fontSize: '0.875rem' }}>
                      {formatTimestamp(ipNet.updated_at)}
                    </TableCell>
                    <TableCell>
                      <Box sx={{ display: 'flex', gap: 1 }}>
                        <Tooltip title="删除此IP或CIDR">
                          <Button
                            variant="outlined"
                            size="small"
                            color="error"
                            startIcon={<DeleteIcon />}
                            onClick={() => deleteIP(ipNet.id)}
                          >
                            删除
                          </Button>
                        </Tooltip>
                        <Tooltip title="修改所属组">
                          <Button
                            variant="outlined"
                            size="small"
                            color="secondary"
                            startIcon={<EditIcon />}
                            onClick={() => openChangeGroupDialog(ipNet)}
                          >
                            修改组
                          </Button>
                        </Tooltip>
                        <Tooltip title="修改行为">
                          <Button
                            variant="outlined"
                            size="small"
                            color="info"
                            startIcon={<EditIcon />}
                            onClick={() => openChangeActionDialog(ipNet)}
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
      </Paper>

      {/* 编辑组对话框 */}
      <Dialog open={!!editGroupId} onClose={() => setEditGroupId('')} maxWidth="sm" fullWidth>
        <DialogTitle>编辑组信息</DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            label="组名"
            value={editGroupName}
            onChange={(e) => setEditGroupName(e.target.value)}
            size="small"
          />
          <TextField
            fullWidth
            label="描述"
            value={editGroupDescription}
            onChange={(e) => setEditGroupDescription(e.target.value)}
            size="small"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditGroupId('')} color="primary">
            取消
          </Button>
          <Button onClick={saveGroup} color="primary" disabled={editGroupLoading}>
            {editGroupLoading ? '保存中...' : '保存'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 修改IP所属组对话框 */}
      <Dialog open={changeGroupDialog} onClose={() => setChangeGroupDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>修改IP所属组</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            当前IP: {selectedIP}
          </Typography>
          <FormControl fullWidth size="small">
            <InputLabel>选择新的组</InputLabel>
            <Select
              value={newGroupId}
              label="选择新的组"
              onChange={(e) => setNewGroupId(e.target.value)}
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
          <Button onClick={() => setChangeGroupDialog(false)} color="primary">
            取消
          </Button>
          <Button onClick={changeIPGroup} color="primary" disabled={changeGroupLoading}>
            {changeGroupLoading ? '修改中...' : '修改'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 修改IP行为对话框 */}
      <Dialog open={changeActionDialog} onClose={() => setChangeActionDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>修改IP行为</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            当前IP: {selectedIP}
          </Typography>
          <FormControl fullWidth size="small">
            <InputLabel>选择新的行为</InputLabel>
            <Select
              value={newAction}
              label="选择新的行为"
              onChange={(e) => setNewAction(e.target.value)}
            >
              {availableActions.map((action) => (
                <MenuItem key={action} value={action}>
                  {getActionDisplayText(action)}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setChangeActionDialog(false)} color="primary">
            取消
          </Button>
          <Button onClick={changeIPAction} color="primary" disabled={changeActionLoading}>
            {changeActionLoading ? '修改中...' : '修改'}
          </Button>
        </DialogActions>
      </Dialog>

      <MessageSnackbar snackbar={snackbar} onClose={hideMessage} />
    </Box>
  );
}

export default IPManagement; 