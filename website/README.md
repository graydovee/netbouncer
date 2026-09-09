# NetBouncer - React + TypeScript 前端

NetBouncer 的前端应用，使用 React 19 + TypeScript + MUI 构建，提供流量监控、IP 规则管理和分组管理界面。

## 功能特性

- 🖥️ **流量监控**: 实时显示网络连接数据，支持多列排序、自动刷新（间隔可配置）
- 🚫 **IP 管理**: 服务端分页列表，支持搜索/按组/按行为过滤，封禁与放行
- 📁 **分组管理**: IP 分组的创建、编辑、删除，实时显示组内 IP 数量
- 🔧 **批量操作**: 批量删除、批量设置行为、批量移动组（走后端批量 API）
- 📥 **批量导入**: 文本粘贴或 URL 拉取批量导入 IP/CIDR
- 🔗 **URL 状态同步**: 分页/筛选/排序状态同步到 URL，刷新不丢失、可分享
- 📱 **响应式设计**: 小屏自动隐藏次要列，支持移动端抽屉导航
- 🔐 **登录支持**: BasicAuth 表单登录与 OIDC 跳转登录，401 自动回到登录页

## 技术栈

- **React 19**: 前端框架
- **TypeScript**: 类型安全（strict 模式）
- **Vite 6**: 构建工具与开发服务器
- **React Router v7**: 路由管理
- **MUI v7**: UI 组件库
- **Vitest**: 单元测试

## 安装和运行

### 前置要求

- Node.js 18+

### 安装依赖

```bash
npm install
```

### 开发模式运行

```bash
# 需要先启动后端（默认代理到 http://localhost:8080）
npm run dev
```

应用将在 `http://localhost:5173` 启动，`/api` 与 `/auth` 请求自动代理到后端（可用环境变量 `VITE_BACKEND_URL` 覆盖）。

### 常用脚本

| 命令 | 说明 |
|------|------|
| `npm run dev` | 启动开发服务器 |
| `npm run build` | 类型检查 + 生产构建（输出到 `dist/`） |
| `npm test` | 运行 Vitest 单元测试 |
| `npm run lint` | ESLint 检查 |
| `npm run typecheck` | 仅 TypeScript 类型检查 |

## 项目结构

```
src/
├── api/                 # 统一 API 层
│   ├── client.ts        # fetch 封装（HTTP 状态码检查、401 统一处理、错误提取）
│   ├── types.ts         # 与后端 Go 结构体对齐的接口定义
│   ├── auth.ts          # 认证接口
│   ├── traffic.ts       # 流量接口
│   ├── ip.ts            # IP 规则接口（含批量/导入）
│   └── group.ts         # 分组接口
├── hooks/
│   ├── useDebounce.ts   # 防抖
│   ├── useMessageSnackbar.ts # 消息提示 hook
│   └── useUrlParams.ts  # URL 查询参数状态同步
├── utils/
│   ├── format.ts        # 字节/速率/时间格式化（含单元测试）
│   └── actions.ts       # 规则动作文案与颜色映射
├── components/          # 通用组件
│   ├── Layout.tsx       # 主布局（侧边栏 + 顶栏）
│   ├── ProtectedRoute.tsx    # 登录保护
│   ├── ConfirmDialog.tsx     # 危险操作确认框
│   ├── MessageSnackbar.tsx   # 消息提示条
│   ├── EmptyState.tsx        # 列表空状态
│   └── RowsPerPageControl.tsx # 每页条数控件
├── context/
│   └── AuthContext.tsx  # 认证状态（登录/登出/401 处理/定时校验）
├── pages/
│   ├── TrafficMonitor.tsx    # 流量监控
│   ├── GroupManagement.tsx   # 组管理
│   ├── Login.tsx / NotFound.tsx
│   └── ip/              # IP 管理模块
│       ├── IPManagement.tsx  # 主页面（服务端分页 + 批量操作）
│       ├── ImportDialog.tsx  # 导入对话框
│       ├── IpRowDialogs.tsx  # 单条规则修改组/行为对话框
│       └── BatchDialogs.tsx  # 批量设置行为/组对话框
├── App.tsx              # 路由与主题
└── main.tsx             # 入口
```

## 开发约定

### API 调用

不要在组件里直接写 `fetch`，统一使用 `src/api/` 下的封装：

```typescript
import { ipApi } from '../api/ip'
import { errorMessage } from '../api/client'

try {
  const result = await ipApi.list({ page: 1, page_size: 20 })
} catch (err) {
  showMessage(errorMessage(err, '获取失败'), 'error')
}
```

封装层已统一处理：HTTP 状态码检查、`{code,message,data}` 解包、401 自动触发重新认证、错误文案提取。

### 列表页状态

分页/筛选/排序使用 `useUrlParams` 同步到 URL，保证刷新不丢失。

### 添加新页面

1. 在 `src/pages/` 下创建页面组件
2. 在 `src/App.tsx` 添加路由
3. 在 `src/components/Layout.tsx` 的 `menuItems` 中添加导航项

## 构建和部署

`make build-web`（仓库根目录）会执行 `npm ci && npm run build`，并把 `dist/` 复制到后端 `web/` 目录；`make all` / Docker 构建会进一步将其嵌入 Go 二进制（`-tags embed`），实现单文件部署。
