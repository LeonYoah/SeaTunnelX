# LINUX DO CDK Frontend

🎨 Linux Do 社区 CDK 快速分享平台 - 前端应用

[![Next.js](https://img.shields.io/badge/Next.js-15-black.svg)](https://nextjs.org/)
[![React](https://img.shields.io/badge/React-19-blue.svg)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-blue.svg)](https://www.typescriptlang.org/)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-4-blue.svg)](https://tailwindcss.com/)

## 📋 目录

- [技术栈](#-技术栈)
- [快速开始](#-快速开始)
- [项目结构](#-项目结构)
- [开发指南](#-开发指南)
- [代码规范](#-代码规范)
- [构建部署](#-构建部署)
- [故障排除](#-故障排除)

## 🛠️ 技术栈

### 核心框架
- **[Next.js 15](https://nextjs.org/)** - React 框架，支持服务端渲染和静态生成
- **[React 19](https://reactjs.org/)** - 用户界面构建库
- **[TypeScript 5](https://www.typescriptlang.org/)** - 静态类型检查

### UI 组件和样式
- **[Tailwind CSS 4](https://tailwindcss.com/)** - 实用优先的 CSS 框架
- **[Shadcn UI](https://ui.shadcn.com/)** - 高质量的 UI 组件集合
- **[Lucide Icons](https://lucide.dev/)** - 简约美观的图标库
- **[Noto Sans SC](https://fonts.google.com/noto/specimen/Noto+Sans+SC)** - 中文字体支持

### 状态管理和数据处理
- **[Axios](https://axios-http.com/)** - HTTP 客户端
- **[Zod](https://zod.dev/)** - TypeScript 优先的模式验证
- **[React Hook Form](https://react-hook-form.com/)** - 高性能表单库

### 开发工具
- **[ESLint](https://eslint.org/)** - 代码质量检查
- **[Prettier](https://prettier.io/)** - 代码格式化
- **[Turbopack](https://turbo.build/pack)** - 高性能构建工具

## 🚀 快速开始

### 环境要求

- **Node.js** >= 18.0
- **pnpm** >= 8.0 (推荐) 或 **npm** >= 9.0

### 安装与启动

```bash
# 安装依赖 (推荐使用 pnpm)
pnpm install

# 开发模式启动 (使用 Turbopack)
pnpm dev

# 访问应用
# 浏览器打开 http://localhost:3000
```

### 其他命令

```bash
# 构建生产版本
pnpm build

# 启动生产服务
pnpm start

# 代码检查
pnpm lint

# 修复 ESLint 错误
pnpm lint:fix

# 代码格式化
pnpm format

# 检查格式化
pnpm format:check
```

## 📁 项目结构

```
frontend/
├── app/                    # Next.js App Router
│   ├── (auth)/            # 认证相关路由组
│   ├── dashboard/         # 仪表板页面
│   ├── globals.css        # 全局样式
│   ├── layout.tsx         # 根布局组件
│   └── page.tsx           # 首页
├── components/            # React 组件
│   ├── common/           # 业务通用组件
│   │   ├── layout/       # 布局组件
│   │   └── forms/        # 表单组件
│   ├── ui/               # Shadcn UI 组件
│   └── icons/            # 自定义图标组件
├── lib/                  # 工具库和配置
│   ├── services/         # API 服务层
│   ├── utils.ts          # 通用工具函数
│   └── constants.ts      # 常量定义
├── public/               # 静态资源
├── types/                # TypeScript 类型定义
└── tailwind.config.js    # Tailwind CSS 配置
```

### 目录说明

| 目录 | 描述 | 规范 |
|------|------|------|
| `app/` | Next.js 15 App Router 页面组件 | 使用文件系统路由 |
| `components/common/` | 业务相关的通用组件 | 按功能模块组织 |
| `components/ui/` | Shadcn UI 基础组件 | 不直接修改，通过覆盖样式自定义 |
| `components/layout/` | 布局相关组件 | 页面结构和导航组件 |
| `components/icons/` | 自定义图标组件 | 命名导出，SVG 组件形式 |
| `lib/services/` | API 服务层 | 按业务领域划分服务 |
| `types/` | TypeScript 类型定义 | 全局类型和接口定义 |

## 🧑‍💻 开发指南

### 开发工作流

1. **启动开发服务器**
   ```bash
   pnpm dev
   ```

2. **创建新组件**
   ```bash
   # 在对应目录创建组件文件
   touch components/common/my-component.tsx
   ```

3. **添加新页面**
   ```bash
   # 在 app 目录下创建路由文件
   mkdir app/my-page
   touch app/my-page/page.tsx
   ```

4. **测试和验证**
   ```bash
   pnpm lint
   pnpm format:check
   pnpm build
   ```

### 服务层架构

服务层是前端与 API 交互的统一入口，基于以下原则：

- **关注点分离** - 每个服务负责一个业务领域
- **统一入口** - 通过 services 对象导出所有服务
- **类型安全** - 所有请求和响应有明确类型定义

#### 创建新服务

1. **创建目录结构**：
   ```
   /lib/services/新服务名/
     ├── types.ts           # 类型定义
     ├── 服务名.service.ts    # 服务实现
     └── index.ts           # 导出服务
   ```

2. **实现服务类**：
   ```typescript
   // 新服务名/服务名.service.ts
   import { BaseService } from '../core/base.service';

   export class 新服务类 extends BaseService {
     protected static readonly basePath = '/api/v1/路径';

     static async 方法名(参数): Promise<返回类型> {
       return this.get<返回类型>('/endpoint');
     }
   }
   ```

3. **在 services/index.ts 注册**：
   ```typescript
   import { 新服务类 } from './新服务名';

   const services = {
     auth: AuthService,
     新服务名: 新服务类
   };
   ```

#### 使用服务

```typescript
import services from '@/lib/services';

// 调用服务方法
const 结果 = await services.新服务名.方法名(参数);
```

## 📐 代码规范

### TypeScript 规范

- **禁止使用 `any` 类型** - 使用具体类型或 `unknown`
- **优先使用接口** - 定义对象结构时使用 `interface`
- **严格类型检查** - 所有组件和函数都要有明确的类型定义

```typescript
// ✅ 好的实践
interface UserProps {
  name: string;
  age: number;
  isActive?: boolean;
}

const UserProfile: React.FC<UserProps> = ({ name, age, isActive = false }) => {
  return <div>{name}</div>;
};

// ❌ 避免的写法
const UserProfile = ({ name, age, isActive }: any) => {
  return <div>{name}</div>;
};
```

### 组件规范

- **函数组件优先** - 使用函数组件和 React Hooks
- **Props 类型定义** - 所有组件都要定义 Props 接口
- **默认导出** - 组件文件使用默认导出

```typescript
// components/common/user-card.tsx
interface UserCardProps {
  user: User;
  onClick?: () => void;
}

export default function UserCard({ user, onClick }: UserCardProps) {
  return (
    <div className="card" onClick={onClick}>
      <h3>{user.name}</h3>
    </div>
  );
}
```

### 样式规范

- **Tailwind 优先** - 优先使用 Tailwind CSS 原子类
- **组件级样式** - 复杂样式使用 CSS Modules 或 styled-components
- **响应式设计** - 使用 Tailwind 的响应式前缀

```tsx
// ✅ 推荐的样式写法
<div className="flex items-center gap-4 p-4 bg-white rounded-lg shadow-sm hover:shadow-md transition-shadow">
  <Avatar className="h-10 w-10" />
  <div className="flex-1">
    <h3 className="font-medium text-gray-900">{user.name}</h3>
    <p className="text-sm text-gray-500">{user.email}</p>
  </div>
</div>
```

### 图标规范

- **Lucide 优先** - 常规图标使用 Lucide React 库
- **自定义图标** - 特殊图标放在 `components/icons/` 目录
- **统一尺寸** - 图标尺寸使用 Tailwind 的 size 类

```tsx
// 使用 Lucide 图标
import { Search, User, Settings } from 'lucide-react';

// 自定义图标
import { LinuxDoLogo } from '@/components/icons';

<Search className="h-5 w-5 text-gray-400" />
```

### 命名规范

| 类型 | 规范 | 示例 |
|------|------|------|
| 文件名 | kebab-case | `user-profile.tsx` |
| 组件名 | PascalCase | `UserProfile` |
| 函数/变量 | camelCase | `getUserData` |
| 常量 | UPPER_SNAKE_CASE | `API_BASE_URL` |
| 类型/接口 | PascalCase | `UserData`, `ApiResponse` |

## 🚀 构建部署

### 本地构建

```bash
# 构建生产版本
pnpm build

# 验证构建结果
pnpm start
```

### 生产环境变量

创建 `.env.production` 文件：

```env
FRONTEND_BASE_URL=http://localhost:3000
BACKEND_BASE_URL=http://localhost:8000
```

### Docker 部署

```dockerfile
# 使用项目根目录的 Dockerfile
# 已包含前端构建配置
```

### 静态导出（可选）

```bash
# 配置 next.config.js
export default {
  output: 'export',
  trailingSlash: true,
};

# 构建静态文件
pnpm build
```

## 🔧 故障排除

### 常见问题

#### 1. 依赖安装失败

```bash
# 清除缓存重新安装
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

#### 2. TypeScript 类型错误

```bash
# 重新生成类型定义
pnpm build
# 或检查 tsconfig.json 配置
```

#### 3. Tailwind 样式不生效

检查 `tailwind.config.js` 的 content 配置：

```javascript
module.exports = {
  content: [
    './app/**/*.{js,ts,jsx,tsx}',
    './components/**/*.{js,ts,jsx,tsx}',
  ],
  // ...
};
```

#### 4. 构建缓存问题

```bash
# 清除 Next.js 缓存
rm -rf .next
pnpm build
```

### 性能优化

- 使用 `next/image` 组件优化图片
- 实现代码分割和懒加载
- 使用 React.memo 优化组件渲染
- 配置 PWA 支持离线访问

### 调试技巧

```bash
# 开启详细日志
DEBUG=* pnpm dev

# 分析构建包大小
pnpm build && npx @next/bundle-analyzer
```

## 🤝 贡献指南

1. Fork 项目并创建特性分支
2. 遵循代码规范和 ESLint 配置
3. 添加必要的测试用例
4. 更新相关文档
5. 提交 Pull Request

更多详细信息请参考项目根目录的 [CONTRIBUTING.md](../CONTRIBUTING.md)。

---

💡 **提示**: 如有问题或建议，欢迎在项目 Issues 中反馈！

