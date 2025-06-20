# 网络监控系统 - React前端

这是网络监控系统的React前端应用，提供了现代化的用户界面来管理网络流量监控和IP禁用功能。

## 功能特性

- 🖥️ **网络流量监控**: 实时显示网络连接数据，支持排序和自动刷新
- 🚫 **IP禁用管理**: 管理已禁用的IP地址列表
- 📱 **响应式设计**: 支持桌面和移动设备
- 🎨 **现代化UI**: 使用Material-UI组件库
- 🔄 **实时更新**: 支持自动刷新和手动刷新
- 📊 **数据排序**: 支持多列数据排序

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
│   └── Layout.jsx      # 主布局组件（包含导航栏）
├── pages/              # 页面组件
│   ├── TrafficMonitor.jsx  # 网络流量监控页面
│   └── BannedIPs.jsx       # 已禁用IP管理页面
├── App.jsx             # 主应用组件
└── main.jsx            # 应用入口
```

## API接口

应用需要后端提供以下API接口：

- `GET /api/traffic` - 获取网络流量数据
- `GET /api/banned` - 获取已禁用IP列表
- `POST /api/ban` - 禁用指定IP
- `POST /api/unban` - 解禁指定IP

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

如果后端服务运行在不同的端口，请相应修改 `target` 地址。

## 使用说明

### 网络流量监控

1. 页面会自动加载网络流量数据
2. 可以设置自动刷新间隔（默认30秒）
3. 点击表头可以按列排序
4. 点击"禁用"按钮可以禁用指定IP

### 已禁用IP管理

1. 查看所有已禁用的IP地址
2. 点击"解禁"按钮可以解除IP禁用
3. 点击"刷新列表"可以手动更新数据

## 开发说明

### 添加新页面

1. 在 `src/pages/` 目录下创建新的页面组件
2. 在 `src/components/Layout.jsx` 中的 `menuItems` 数组中添加菜单项
3. 在 `src/App.jsx` 中添加对应的路由

### 自定义主题

可以在 `src/App.jsx` 中修改 `theme` 对象来自定义Material-UI主题。

## 故障排除

### 常见问题

1. **API请求失败**: 检查后端服务是否运行，以及代理配置是否正确
2. **页面无法加载**: 确保所有依赖已正确安装
3. **样式问题**: 确保Material-UI依赖已正确安装

### 调试

- 使用浏览器开发者工具查看网络请求和控制台错误
- 检查Vite开发服务器的代理配置
- 确认后端API接口返回正确的JSON格式

# 前端项目使用说明

## 开发模式

### 使用默认后端地址（localhost:8080）
```bash
make web-dev
```

### 使用自定义后端地址
```bash
VITE_BACKEND_URL=http://192.168.1.100:8080 make web-dev
```

### 直接使用npm
```bash
cd website
VITE_BACKEND_URL=http://192.168.1.100:8080 npm run dev
```

## 构建生产版本

```bash
make build-web
```

构建后的文件会自动复制到 `web/` 目录，供Go后端服务。

## 环境变量

- `VITE_BACKEND_URL`: 后端服务地址，默认为 `http://localhost:8080`

## 注意事项

1. 开发模式下，前端会通过Vite的proxy功能代理API请求到后端
2. 生产构建时，API请求使用相对路径（如 `/api/traffic`），由Go后端直接处理
3. 前端代码中的API调用都是相对路径，无需修改
