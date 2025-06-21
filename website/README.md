# 网络监控系统 - React前端

这是网络监控系统的React前端应用，提供了现代化的用户界面来管理网络流量监控和IP管理功能。

## 功能特性

- 🖥️ **网络流量监控**: 实时显示网络连接数据，支持排序和自动刷新
- 🚫 **IP管理**: 管理IP地址列表，支持封禁和允许两种行为
- 📁 **分组管理**: 支持IP分组管理，便于批量操作
- 📱 **响应式设计**: 支持桌面和移动设备
- 🎨 **现代化UI**: 使用Material-UI组件库
- 🔄 **实时更新**: 支持自动刷新和手动刷新
- 📊 **数据排序**: 支持多列数据排序
- 🔧 **批量操作**: 支持批量修改IP行为和所属组
- 📥 **批量导入**: 支持批量导入IP地址

## 技术栈

- **React 18**: 前端框架
- **Vite**: 构建工具
- **React Router**: 路由管理
- **Material-UI**: UI组件库
- **Emotion**: CSS-in-JS解决方案

## 安装和运行

### 前置要求

- Node.js 16+ 
- npm 或 yarn

### 安装依赖

```bash
npm install
```

### 开发模式运行

```bash
npm run dev
```

应用将在 `http://localhost:5173` 启动。

### 构建生产版本

```bash
npm run build
```

构建文件将生成在 `dist` 目录中。

### 预览生产版本

```bash
npm run preview
```

## 项目结构

```
src/
├── components/          # 通用组件
│   ├── Layout.jsx      # 主布局组件（包含导航栏）
│   ├── ConfirmDialog.jsx # 确认对话框组件
│   └── MessageSnackbar.jsx # 消息提示组件
├── pages/              # 页面组件
│   ├── TrafficMonitor.jsx  # 网络流量监控页面
│   ├── IPManagement.jsx    # IP管理页面
│   ├── GroupManagement.jsx # 组管理页面
│   └── NotFound.jsx        # 404页面
├── App.jsx             # 主应用组件
└── main.jsx            # 应用入口
```

## API接口

应用需要后端提供以下API接口：

### 流量监控
- `GET /api/traffic` - 获取网络流量数据

### IP管理
- `GET /api/ip` - 获取所有IP列表
- `GET /api/ip/:groupId` - 根据组ID获取IP列表
- `POST /api/ip` - 创建IP规则
- `DELETE /api/ip/:id` - 删除IP规则
- `GET /api/ip/action` - 获取可用操作列表
- `PUT /api/ip/action` - 更新IP行为
- `PUT /api/ip/group` - 更新IP所属组

### 组管理
- `GET /api/group` - 获取所有组列表
- `POST /api/group` - 创建新组
- `PUT /api/group` - 更新组信息
- `DELETE /api/group/:id` - 删除组

## 配置

### 代理配置

在 `vite.config.js` 中配置了API代理，将 `/api` 请求代理到后端服务器：

```javascript
server: {
  proxy: {
    '/api': {
      target: 'http://localhost:8080',
      changeOrigin: true,
    },
  },
}
```

### 环境变量

可以通过环境变量配置后端地址：

```bash
# 设置后端地址
export VITE_BACKEND_URL=http://localhost:8080

# 启动开发服务器
npm run dev
```

## 页面功能说明

### 流量监控页面 (TrafficMonitor)

- **实时流量显示**: 显示所有网络连接的流量统计
- **数据排序**: 支持按流量、连接数等字段排序
- **自动刷新**: 可配置自动刷新间隔
- **一键封禁**: 点击按钮快速封禁IP
- **状态显示**: 显示IP是否已被封禁

### IP管理页面 (IPManagement)

- **IP列表管理**: 查看所有IP或按组查看
- **创建IP规则**: 添加新的IP地址或CIDR网段
- **删除IP规则**: 删除不需要的IP规则
- **修改IP行为**: 在封禁和允许之间切换
- **修改所属组**: 将IP移动到不同的组
- **批量操作**: 批量修改IP行为和所属组
- **批量导入**: 支持批量导入IP地址列表

### 组管理页面 (GroupManagement)

- **组列表**: 显示所有IP分组
- **创建组**: 创建新的IP分组
- **编辑组**: 修改组名称和描述
- **删除组**: 删除不需要的组

## 组件说明

### Layout组件

主布局组件，包含：
- 响应式侧边栏导航
- 顶部应用栏
- 移动端适配

### ConfirmDialog组件

确认对话框组件，用于：
- 删除确认
- 危险操作确认
- 自定义确认消息

### MessageSnackbar组件

消息提示组件，用于：
- 操作成功提示
- 错误信息显示
- 警告信息显示

## 开发指南

### 添加新页面

1. 在 `src/pages/` 目录下创建新的页面组件
2. 在 `src/App.jsx` 中添加路由
3. 在 `src/components/Layout.jsx` 中添加导航菜单项

### 添加新API调用

1. 在页面组件中添加API调用函数
2. 使用 `fetch` 或 `axios` 进行HTTP请求
3. 处理响应数据和错误情况

### 样式定制

项目使用Material-UI主题系统，可以通过以下方式定制样式：

1. 修改 `src/App.jsx` 中的主题配置
2. 使用 `sx` 属性进行内联样式
3. 使用 `styled` 组件创建自定义组件

## 构建和部署

### 构建生产版本

```bash
npm run build
```

### 部署到静态服务器

将 `dist` 目录中的文件部署到任何静态文件服务器即可。

### Docker部署

```dockerfile
FROM nginx:alpine
COPY dist/ /usr/share/nginx/html/
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

## 故障排除

### 开发环境问题

1. **端口冲突**: 修改 `vite.config.js` 中的端口配置
2. **API代理失败**: 检查后端服务是否正常运行
3. **依赖安装失败**: 清除 `node_modules` 重新安装

### 生产环境问题

1. **路由404**: 确保服务器配置了正确的重写规则
2. **API请求失败**: 检查CORS配置和API地址
3. **静态资源加载失败**: 检查构建路径配置

## 贡献指南

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 推送到分支
5. 创建 Pull Request

## 许可证

本项目采用 MIT 许可证。
