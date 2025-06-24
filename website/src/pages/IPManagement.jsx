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
  Checkbox,
  FormControlLabel,
  TablePagination,
  Radio,
  RadioGroup,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  Block as BlockIcon,
  Add as AddIcon,
  FilterList as FilterIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  SelectAll as SelectAllIcon,
  Clear as ClearIcon,
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
  
  // 批量操作相关状态
  const [selectedIPs, setSelectedIPs] = useState(new Set());
  const [batchDeleteDialog, setBatchDeleteDialog] = useState(false);
  const [batchActionDialog, setBatchActionDialog] = useState(false);
  const [batchGroupDialog, setBatchGroupDialog] = useState(false);
  const [batchAction, setBatchAction] = useState('ban');
  const [batchGroupId, setBatchGroupId] = useState('');
  const [batchLoading, setBatchLoading] = useState(false);
  
  // 批量导入相关状态
  const [batchImportDialog, setBatchImportDialog] = useState(false);
  const [importText, setImportText] = useState('');
  const [importUrl, setImportUrl] = useState('');
  const [importAction, setImportAction] = useState('ban');
  const [importGroupId, setImportGroupId] = useState('');
  const [importLoading, setImportLoading] = useState(false);
  const [importMode, setImportMode] = useState('text'); // 'text' 或 'url'
  
  // 分页相关状态
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(25);
  const [customRowsPerPage, setCustomRowsPerPage] = useState(25);
  
  // 过滤相关状态
  const [filterAction, setFilterAction] = useState('');
  const [filterAddress, setFilterAddress] = useState('');
  const [showFilters, setShowFilters] = useState(false);
  
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
    // 清空选择、分页和过滤
    setSelectedIPs(new Set());
    setPage(0);
    clearFilters();
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

  // 分页处理函数
  const handleChangePage = (event, newPage) => {
    setPage(newPage);
  };

  const handleChangeRowsPerPage = (event) => {
    const newRowsPerPage = parseInt(event.target.value, 10);
    if (newRowsPerPage > 0) {
      setRowsPerPage(newRowsPerPage);
      setCustomRowsPerPage(newRowsPerPage);
      setPage(0);
    }
  };

  const handleCustomRowsPerPageChange = (event) => {
    const value = parseInt(event.target.value, 10);
    setCustomRowsPerPage(value);
  };

  const handleCustomRowsPerPageBlur = () => {
    const value = customRowsPerPage;
    if (value > 0 && value <= 10000) {
      setRowsPerPage(value);
      setPage(0);
    } else {
      // 如果输入的值无效，重置为当前有效的分页数量
      setCustomRowsPerPage(rowsPerPage);
    }
  };

  const handleCustomRowsPerPageKeyDown = (event) => {
    if (event.key === 'Enter') {
      event.target.blur(); // 触发失去焦点事件
    }
  };

  // 过滤数据
  const getFilteredData = () => {
    let filtered = [...ipNets];
    
    // 按行为过滤（精确匹配）
    if (filterAction) {
      filtered = filtered.filter(ip => ip.action === filterAction);
    }
    
    // 按地址过滤（模糊匹配）
    if (filterAddress) {
      filtered = filtered.filter(ip => 
        ip.ip_net.toLowerCase().includes(filterAddress.toLowerCase())
      );
    }
    
    return filtered;
  };

  // 获取当前页的数据
  const getCurrentPageData = () => {
    const filteredData = getFilteredData();
    const startIndex = page * rowsPerPage;
    const endIndex = startIndex + rowsPerPage;
    return filteredData.slice(startIndex, endIndex);
  };

  // 获取当前页选中的IP
  const getCurrentPageSelectedIPs = () => {
    const currentPageData = getCurrentPageData();
    return new Set(Array.from(selectedIPs).filter(ipId => 
      currentPageData.some(ip => ip.id === ipId)
    ));
  };

  // 处理全选当前页
  const handleSelectAllCurrentPage = () => {
    const currentPageData = getCurrentPageData();
    const currentPageSelected = getCurrentPageSelectedIPs();
    
    if (currentPageSelected.size === currentPageData.length) {
      // 取消选择当前页所有项
      const newSelected = new Set(selectedIPs);
      currentPageData.forEach(ip => newSelected.delete(ip.id));
      setSelectedIPs(newSelected);
    } else {
      // 选择当前页所有项
      const newSelected = new Set(selectedIPs);
      currentPageData.forEach(ip => newSelected.add(ip.id));
      setSelectedIPs(newSelected);
    }
  };

  // 清除过滤条件
  const clearFilters = () => {
    setFilterAction('');
    setFilterAddress('');
    setPage(0);
  };

  // 获取过滤后的数据总数
  const getFilteredDataCount = () => {
    return getFilteredData().length;
  };

  // 初始化
  useEffect(() => {
    fetchGroups();
    fetchIpNets();
    fetchAvailableActions();
  }, []);

  // 当组列表加载完成后，设置默认的导入组ID
  useEffect(() => {
    if (groups.length > 0 && !importGroupId) {
      setImportGroupId(groups[0].id);
    }
  }, [groups, importGroupId]);

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

  // 批量操作相关函数
  const handleSelectIP = (ipId) => {
    const newSelected = new Set(selectedIPs);
    if (newSelected.has(ipId)) {
      newSelected.delete(ipId);
    } else {
      newSelected.add(ipId);
    }
    setSelectedIPs(newSelected);
  };

  const clearSelection = () => {
    setSelectedIPs(new Set());
  };

  // 批量删除
  const handleBatchDelete = async () => {
    if (selectedIPs.size === 0) {
      showMessage('请选择要删除的IP', 'warning');
      return;
    }

    setBatchLoading(true);
    let successCount = 0;
    let failCount = 0;

    for (const ipId of selectedIPs) {
      try {
        const response = await fetch(`/api/ip/${ipId}`, {
          method: 'DELETE',
          headers: { 'Content-Type': 'application/json' }
        });
        const result = await response.json();
        if (result.code === 200) {
          successCount++;
        } else {
          failCount++;
        }
      } catch (error) {
        failCount++;
      }
    }

    setBatchLoading(false);
    setBatchDeleteDialog(false);
    setSelectedIPs(new Set());

    if (failCount === 0) {
      showMessage(`成功删除 ${successCount} 个IP`);
    } else {
      showMessage(`删除完成：成功 ${successCount} 个，失败 ${failCount} 个`, failCount > 0 ? 'warning' : 'success');
    }

    // 重新获取列表
    if (selectedTab === 0) {
      await fetchIpNets();
    } else {
      await fetchIpNetsByGroup(selectedGroupId);
    }
  };

  // 批量设置行为
  const handleBatchSetAction = async () => {
    if (selectedIPs.size === 0) {
      showMessage('请选择要设置的IP', 'warning');
      return;
    }

    setBatchLoading(true);
    let successCount = 0;
    let failCount = 0;

    for (const ipId of selectedIPs) {
      try {
        const response = await fetch('/api/ip/action', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ 
            id: ipId,
            action: batchAction
          })
        });
        const result = await response.json();
        if (result.code === 200) {
          successCount++;
        } else {
          failCount++;
        }
      } catch (error) {
        failCount++;
      }
    }

    setBatchLoading(false);
    setBatchActionDialog(false);
    setSelectedIPs(new Set());

    if (failCount === 0) {
      showMessage(`成功设置 ${successCount} 个IP的行为为 ${getActionDisplayText(batchAction)}`);
    } else {
      showMessage(`设置完成：成功 ${successCount} 个，失败 ${failCount} 个`, failCount > 0 ? 'warning' : 'success');
    }

    // 重新获取列表
    if (selectedTab === 0) {
      await fetchIpNets();
    } else {
      await fetchIpNetsByGroup(selectedGroupId);
    }
  };

  // 批量设置组
  const handleBatchSetGroup = async () => {
    if (selectedIPs.size === 0) {
      showMessage('请选择要设置的IP', 'warning');
      return;
    }

    if (!batchGroupId) {
      showMessage('请选择要设置的组', 'warning');
      return;
    }

    setBatchLoading(true);
    let successCount = 0;
    let failCount = 0;

    for (const ipId of selectedIPs) {
      try {
        const response = await fetch('/api/ip/group', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ 
            id: ipId,
            group_id: parseInt(batchGroupId)
          })
        });
        const result = await response.json();
        if (result.code === 200) {
          successCount++;
        } else {
          failCount++;
        }
      } catch (error) {
        failCount++;
      }
    }

    setBatchLoading(false);
    setBatchGroupDialog(false);
    setSelectedIPs(new Set());

    const selectedGroup = groups.find(g => g.id === parseInt(batchGroupId));
    const groupName = selectedGroup ? selectedGroup.name : batchGroupId;

    if (failCount === 0) {
      showMessage(`成功设置 ${successCount} 个IP的组为 ${groupName}`);
    } else {
      showMessage(`设置完成：成功 ${successCount} 个，失败 ${failCount} 个`, failCount > 0 ? 'warning' : 'success');
    }

    // 重新获取列表
    if (selectedTab === 0) {
      await fetchIpNets();
    } else {
      await fetchIpNetsByGroup(selectedGroupId);
    }
  };

  // 批量导入IP
  const handleBatchImport = async () => {
    if (!importText.trim() && !importUrl.trim()) {
      showMessage('请输入要导入的IP或CIDR或URL', 'warning');
      return;
    }

    if (!importGroupId) {
      showMessage('请选择要添加到的组', 'warning');
      return;
    }

    setImportLoading(true);
    try {
      const requestBody = {
        group_id: parseInt(importGroupId),
        action: importAction
      };

      // 根据模式设置请求参数
      if (importMode === 'text' && importText.trim()) {
        requestBody.text = importText.trim();
      } else if (importMode === 'url' && importUrl.trim()) {
        requestBody.url = importUrl.trim();
      } else if (importText.trim() && importUrl.trim()) {
        // 如果两个都填了，优先使用URL
        requestBody.url = importUrl.trim();
      } else if (importText.trim()) {
        requestBody.text = importText.trim();
      } else {
        showMessage('请输入要导入的内容', 'warning');
        setImportLoading(false);
        return;
      }

      const response = await fetch('/api/ip/import', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestBody)
      });
      
      const result = await response.json();
      if (result.code === 200) {
        const responseData = result.data;
        if (responseData.success_count > 0) {
          showMessage(`成功导入 ${responseData.success_count} 个IP${responseData.failed_count > 0 ? `，失败 ${responseData.failed_count} 个` : ''}`);
        } else {
          showMessage('没有成功导入任何IP', 'warning');
        }
        
        // 重新获取列表
        if (selectedTab === 0) {
          await fetchIpNets();
        } else {
          await fetchIpNetsByGroup(selectedGroupId);
        }
      } else {
        showMessage('导入失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('导入失败: 网络错误', 'error');
    } finally {
      setImportLoading(false);
      setBatchImportDialog(false);
      setImportText('');
      setImportUrl('');
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
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
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
          
          {/* 过滤按钮 */}
          <Button
            variant="outlined"
            onClick={() => setShowFilters(!showFilters)}
            startIcon={<FilterIcon />}
            color={showFilters ? 'primary' : 'default'}
          >
            过滤
          </Button>
          
          {/* 批量操作按钮 */}
          {selectedIPs.size > 0 && (
            <>
              <Button
                variant="outlined"
                color="error"
                startIcon={<DeleteIcon />}
                onClick={() => setBatchDeleteDialog(true)}
                disabled={batchLoading}
              >
                批量删除 ({selectedIPs.size})
              </Button>
              <Button
                variant="outlined"
                color="info"
                startIcon={<EditIcon />}
                onClick={() => setBatchActionDialog(true)}
                disabled={batchLoading}
              >
                批量设置行为 ({selectedIPs.size})
              </Button>
              <Button
                variant="outlined"
                color="secondary"
                startIcon={<EditIcon />}
                onClick={() => setBatchGroupDialog(true)}
                disabled={batchLoading}
              >
                批量设置组 ({selectedIPs.size})
              </Button>
              <Button
                variant="outlined"
                color="default"
                startIcon={<ClearIcon />}
                onClick={clearSelection}
                disabled={batchLoading}
              >
                清除选择
              </Button>
            </>
          )}
          
          {/* 批量导入按钮 */}
          <Button
            variant="outlined"
            color="primary"
            startIcon={<AddIcon />}
            onClick={() => setBatchImportDialog(true)}
            disabled={batchLoading || importLoading}
          >
            批量导入
          </Button>
          
          <Typography variant="body2" color="text.secondary">
            共 {getFilteredDataCount()} 个IP
            {selectedTab > 0 && groups[selectedTab - 1] && (
              <span>（组：{groups[selectedTab - 1].name}）</span>
            )}
            {selectedIPs.size > 0 && (
              <span>，已选择 {selectedIPs.size} 个</span>
            )}
            {getFilteredDataCount() > 0 && (
              <span>，当前页 {page + 1}/{Math.ceil(getFilteredDataCount() / rowsPerPage)}</span>
            )}
            {(filterAction || filterAddress) && (
              <span>（已过滤）</span>
            )}
          </Typography>
        </Box>

        {/* 过滤面板 */}
        {showFilters && (
          <Box sx={{ mt: 2, p: 2, border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
            <Typography variant="subtitle2" gutterBottom>
              过滤条件
            </Typography>
            <Grid container spacing={2} alignItems="center">
              <Grid item xs={12} sm={5}>
                <FormControl fullWidth size="small">
                  <InputLabel shrink={true}>行为</InputLabel>
                  <Select
                    value={filterAction}
                    label="行为"
                    onChange={(e) => {
                      setFilterAction(e.target.value);
                      setPage(0);
                    }}
                    displayEmpty
                  >
                    <MenuItem value="">全部</MenuItem>
                    {availableActions.map((action) => (
                      <MenuItem key={action} value={action}>
                        {getActionDisplayText(action)}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              </Grid>
              <Grid item xs={12} sm={4}>
                <TextField
                  fullWidth
                  size="small"
                  label="地址"
                  placeholder="例如：192.168, 10.0"
                  value={filterAddress}
                  onChange={(e) => {
                    setFilterAddress(e.target.value);
                    setPage(0);
                  }}
                />
              </Grid>
              <Grid item xs={12} sm={3}>
                <Button
                  variant="outlined"
                  size="small"
                  onClick={clearFilters}
                  disabled={!filterAction && !filterAddress}
                >
                  清除
                </Button>
              </Grid>
            </Grid>
          </Box>
        )}

        {/* 分页设置 */}
        <Box sx={{ mt: 2, display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
          <Typography variant="body2" color="text.secondary">
            每页显示：
          </Typography>
          <FormControl size="small" sx={{ minWidth: 120 }}>
            <Select
              value={rowsPerPage}
              onChange={handleChangeRowsPerPage}
              displayEmpty
            >
              <MenuItem value={10}>10 条</MenuItem>
              <MenuItem value={25}>25 条</MenuItem>
              <MenuItem value={50}>50 条</MenuItem>
              <MenuItem value={100}>100 条</MenuItem>
            </Select>
          </FormControl>
          <TextField
            size="small"
            type="number"
            label="自定义数量"
            value={customRowsPerPage}
            onChange={handleCustomRowsPerPageChange}
            onBlur={handleCustomRowsPerPageBlur}
            onKeyDown={handleCustomRowsPerPageKeyDown}
            inputProps={{ min: 1, max: 10000 }}
            sx={{ width: 120 }}
          />
        </Box>
      </Paper>

      {/* IP列表表格 */}
      <Paper>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell padding="checkbox">
                  <Checkbox
                    indeterminate={getCurrentPageSelectedIPs().size > 0 && getCurrentPageSelectedIPs().size < getCurrentPageData().length}
                    checked={getCurrentPageData().length > 0 && getCurrentPageSelectedIPs().size === getCurrentPageData().length}
                    onChange={handleSelectAllCurrentPage}
                    disabled={loading || getCurrentPageData().length === 0}
                  />
                </TableCell>
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
                  <TableCell colSpan={7} align="center">
                    <CircularProgress size={24} />
                    <Typography sx={{ ml: 1 }}>加载中...</Typography>
                  </TableCell>
                </TableRow>
              ) : getCurrentPageData().length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} align="center">
                    {selectedTab === 0 ? '暂无IP记录' : '该组暂无IP记录'}
                  </TableCell>
                </TableRow>
              ) : (
                getCurrentPageData().map((ipNet, index) => (
                  <TableRow key={index} hover>
                    <TableCell padding="checkbox">
                      <Checkbox
                        checked={selectedIPs.has(ipNet.id)}
                        onChange={() => handleSelectIP(ipNet.id)}
                        disabled={batchLoading}
                      />
                    </TableCell>
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
                            disabled={batchLoading}
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
                            disabled={batchLoading}
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
                            disabled={batchLoading}
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
        
        {/* 分页组件 */}
        <TablePagination
          component="div"
          count={getFilteredDataCount()}
          page={page}
          onPageChange={handleChangePage}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={[10, 25, 50, 100, customRowsPerPage].filter((value, index, self) => self.indexOf(value) === index).sort((a, b) => a - b)}
          labelRowsPerPage="每页显示:"
          labelDisplayedRows={({ from, to, count }) => `${from}-${to} / ${count}`}
          showFirstButton
          showLastButton
        />
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

      {/* 批量删除确认对话框 */}
      <Dialog open={batchDeleteDialog} onClose={() => setBatchDeleteDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>确认批量删除</DialogTitle>
        <DialogContent>
          <Typography variant="body1" sx={{ mb: 2 }}>
            确定要删除选中的 {selectedIPs.size} 个IP吗？
          </Typography>
          <Typography variant="body2" color="text.secondary">
            此操作不可撤销，请谨慎操作。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setBatchDeleteDialog(false)} disabled={batchLoading}>
            取消
          </Button>
          <Button onClick={handleBatchDelete} color="error" disabled={batchLoading}>
            {batchLoading ? '删除中...' : '确认删除'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 批量设置行为对话框 */}
      <Dialog open={batchActionDialog} onClose={() => setBatchActionDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>批量设置行为</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            为选中的 {selectedIPs.size} 个IP设置行为
          </Typography>
          <FormControl fullWidth size="small">
            <InputLabel>选择行为</InputLabel>
            <Select
              value={batchAction}
              label="选择行为"
              onChange={(e) => setBatchAction(e.target.value)}
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
          <Button onClick={() => setBatchActionDialog(false)} disabled={batchLoading}>
            取消
          </Button>
          <Button onClick={handleBatchSetAction} color="primary" disabled={batchLoading}>
            {batchLoading ? '设置中...' : '确认设置'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 批量设置组对话框 */}
      <Dialog open={batchGroupDialog} onClose={() => setBatchGroupDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>批量设置组</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            为选中的 {selectedIPs.size} 个IP设置组
          </Typography>
          <FormControl fullWidth size="small">
            <InputLabel>选择组</InputLabel>
            <Select
              value={batchGroupId}
              label="选择组"
              onChange={(e) => setBatchGroupId(e.target.value)}
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
          <Button onClick={() => setBatchGroupDialog(false)} disabled={batchLoading}>
            取消
          </Button>
          <Button onClick={handleBatchSetGroup} color="primary" disabled={batchLoading}>
            {batchLoading ? '设置中...' : '确认设置'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 批量导入对话框 */}
      <Dialog open={batchImportDialog} onClose={() => setBatchImportDialog(false)} maxWidth="md" fullWidth>
        <DialogTitle>批量导入IP</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            请选择导入方式并输入要导入的IP地址或CIDR
          </Typography>
          
          {/* 导入模式选择 */}
          <Box sx={{ mb: 2 }}>
            <FormControl component="fieldset">
              <RadioGroup
                value={importMode}
                onChange={(e) => setImportMode(e.target.value)}
                row
              >
                <FormControlLabel
                  value="text"
                  control={<Radio />}
                  label="文本输入"
                />
                <FormControlLabel
                  value="url"
                  control={<Radio />}
                  label="URL导入"
                />
              </RadioGroup>
            </FormControl>
          </Box>
          
          {/* 文本输入 */}
          {importMode === 'text' && (
            <TextField
              fullWidth
              multiline
              rows={8}
              label="IP地址或CIDR列表"
              placeholder="例如：192.168.1.1, 10.0.0.0/8, 172.16.0.1"
              value={importText}
              onChange={(e) => setImportText(e.target.value)}
              disabled={importLoading}
              sx={{ mb: 2 }}
            />
          )}
          
          {/* URL输入 */}
          {importMode === 'url' && (
            <TextField
              fullWidth
              label="URL地址"
              placeholder="例如：https://example.com/ip-list.txt"
              value={importUrl}
              onChange={(e) => setImportUrl(e.target.value)}
              disabled={importLoading}
              sx={{ mb: 2 }}
            />
          )}
          
          <Grid container spacing={2}>
            <Grid item xs={12} sm={6}>
              <FormControl fullWidth size="small" disabled={importLoading || groupsLoading}>
                <InputLabel>选择组</InputLabel>
                <Select
                  value={importGroupId}
                  label="选择组"
                  onChange={(e) => setImportGroupId(e.target.value)}
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
            <Grid item xs={12} sm={6}>
              <FormControl fullWidth size="small" disabled={importLoading}>
                <InputLabel>选择行为</InputLabel>
                <Select
                  value={importAction}
                  label="选择行为"
                  onChange={(e) => setImportAction(e.target.value)}
                >
                  {availableActions.map((action) => (
                    <MenuItem key={action} value={action}>
                      {getActionDisplayText(action)}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          </Grid>
          
          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            {importMode === 'text' ? (
              '支持格式：单个IP（如：192.168.1.1）或CIDR网段（如：192.168.1.0/24），多个IP请用逗号分隔'
            ) : (
              'URL应返回纯文本格式的IP列表，每行一个IP或CIDR，支持注释行（以#开头）'
            )}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setBatchImportDialog(false)} disabled={importLoading}>
            取消
          </Button>
          <Button 
            onClick={handleBatchImport} 
            color="primary" 
            disabled={importLoading || (!importText.trim() && !importUrl.trim()) || !importGroupId}
          >
            {importLoading ? '导入中...' : '确认导入'}
          </Button>
        </DialogActions>
      </Dialog>

      <MessageSnackbar snackbar={snackbar} onClose={hideMessage} />
    </Box>
  );
}

export default IPManagement; 