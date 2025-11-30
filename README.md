# 🌟 SeedParser 种子解析器

## 📋 项目概述

SeedParser 是一个功能强大的种子文件解析工具，支持种子文件的解析、转换和管理。采用现代化的技术栈，提供直观易用的用户界面和强大的后端处理能力。

## 📸 项目截图演示

### 主界面
![主界面](images/1.png)
*简洁明了的主界面设计，支持拖拽上传种子文件*

### 种子文件解析
![文件解析](images/2.png)
*快速解析种子文件，展示详细的文件结构和大小信息*

### 批量处理
![批量处理](images/3.png)
*支持多个种子文件的批量解析和管理功能*

### 高级功能
![高级功能](images/5.png)
*丰富的高级功能，包括数据导出和进度跟踪*

### 主题设置
![主题设置](images/6.png)
*支持亮色/暗色主题切换，提供更好的用户体验*

## ✨ 主要特性

### 🚀 核心功能
- **种子文件解析**：快速解析 .torrent 文件，提取详细信息
- **文件结构预览**：清晰展示种子内文件结构和大小
- **批量处理**：支持多个种子文件的批量解析和管理
- **数据导出**：支持多种格式的数据导出功能
- **进度跟踪**：实时显示解析进度和处理状态

### 🎯 用户体验
- **现代化界面**：基于 Vue 3 + TypeScript 构建的响应式界面
- **多语言支持**：内置国际化支持，支持中英文切换
- **主题定制**：支持亮色/暗色主题切换
- **响应式设计**：适配桌面、平板和移动设备

### ⚡ 性能特性
- **高效解析**：优化的解析算法，快速处理大型种子文件
- **内存优化**：智能内存管理，处理大文件时保持系统稳定
- **异步处理**：非阻塞式操作，提升用户体验

## 🏗️ 技术架构

### 前端技术栈
- **框架**：Vue 3 (Composition API)
- **语言**：TypeScript
- **构建工具**：Vite
- **UI 框架**：Tailwind CSS
- **状态管理**：Pinia
- **路由管理**：Vue Router
- **HTTP 客户端**：Axios
- **国际化**：Vue I18n

### 后端技术栈
- **语言**：Go
- **框架**：Wails (Go + Web 技术栈)
- **网络**：标准库 net/http
- **文件处理**：标准库 io/ioutil, os
- **JSON 处理**：标准库 encoding/json
- **并发处理**：Goroutines + Channels

### 开发工具
- **代码规范**：ESLint + Prettier
- **包管理**：npm (前端) + Go Modules (后端)
- **版本控制**：Git
- **构建打包**：Wails Build System

## 📁 项目结构

```
SeedParser/
├── 📁 frontend/                 # 前端源码
│   ├── 📁 src/                  # Vue 源码
│   │   ├── 📁 components/       # 组件库
│   │   ├── 📁 views/            # 页面视图
│   │   ├── 📁 stores/           # 状态管理
│   │   ├── 📁 router/           # 路由配置
│   │   ├── 📁 i18n/             # 国际化文件
│   │   └── 📁 style/            # 样式文件
│   ├── 📁 public/               # 静态资源
│   ├── 📄 package.json          # 前端依赖
│   ├── 📄 vite.config.ts        # Vite 配置
│   ├── 📄 tailwind.config.cjs   # Tailwind 配置
│   └── 📄 tsconfig.json         # TypeScript 配置
├── 📁 backend/                  # 后端源码 (可选)
│   ├── 📄 app.go                # 主应用程序
│   ├── 📄 main.go               # 程序入口
│   └── 📄 wails.json            # Wails 配置
├── 📁 build/                    # 构建输出
│   ├── 📁 windows/              # Windows 构建配置
│   └── 📁 bin/                  # 可执行文件
├── 📁 tools/                    # 工具依赖
│   └── 📁 ffmpeg/               # FFmpeg 工具链
├── 📁 goujian/                  # 安装器脚本
│   └── 📄 种子解析器.iss         # Inno Setup 脚本
├── 📄 wails.json                # Wails 项目配置
├── 📄 go.mod                    # Go 模块定义
└── 📄 README.md                 # 项目说明
```

## 🛠️ 安装和使用

### 系统要求
- **操作系统**：Windows 10/11 (64位)
- **内存**：至少 4GB RAM
- **硬盘空间**：至少 100MB 可用空间
- **网络**：Internet 连接（用于下载依赖）

### 快速开始

#### 方法一：使用预构建安装器
![下载发行版](images/7.png)
*从发行版页面下载 Windows 构建版本*

1. 下载 `SeedParser-Setup-v1.0.exe`
2. 运行安装器，按照向导完成安装
3. 启动 SeedParser 应用程序

#### 方法二：从源码构建
```bash
# 克隆项目
git clone [项目地址]
cd SeedParser

# 构建前端
cd frontend
npm install
npm run build

# 构建后端 (需要安装 Wails)
wails build

# 运行应用
wails dev
```

## 📦 构建和分发

### 安装器制作
项目包含完整的安装器制作系统：

- **Inno Setup 脚本**：`goujian/种子解析器.iss`
- **NSIS 脚本**：自动化脚本支持
- **批处理工具**：一键生成安装器
- **自动化脚本**：支持 CI/CD 集成

### 安装器特性
- ✅ 自动创建桌面快捷方式
- ✅ 添加开始菜单项
- ✅ 注册卸载程序
- ✅ 文件关联设置
- ✅ 使用条款确认
- ✅ 自动依赖检查

## 🔧 开发指南

### 环境配置
```bash
# 安装 Wails
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 安装前端依赖
cd frontend
npm install

# 启动开发服务器
npm run dev

# 启动应用开发模式
wails dev
```

### 代码规范
- **前端**：遵循 ESLint + Prettier 配置
- **后端**：遵循 Go 官方规范
- **提交信息**：使用约定式提交格式

### 测试
```bash
# 前端测试
cd frontend
npm run test

# 后端测试
go test ./...
```

## 🌐 国际化支持

支持多语言界面：
- 🇨🇳 简体中文 (默认)
- 🇺🇸 English
- 🗣️ 可扩展其他语言支持

## 🔐 安全性

- **使用条款保护**：包含完整的免责声明
- **代码签名**：支持可执行文件签名
- **安全验证**：文件完整性检查
- **隐私保护**：不收集用户隐私数据

## 📄 许可证

本项目基于 [LICENSE.txt](LICENSE.txt) 许可证发布。

**重要声明**：任何非法传播和使用与作者无关。

## 🤝 贡献指南

欢迎贡献代码、报告问题或提出功能建议！

### 贡献流程
1. Fork 项目
2. 创建特性分支
3. 提交更改
4. 推送到分支
5. 创建 Pull Request

### 报告问题
请在 GitHub Issues 中报告问题，包含：
- 详细的问题描述
- 复现步骤
- 系统环境信息
- 错误日志

## 📞 支持和反馈

- **项目主页**：[GitHub 仓库链接]
- **问题反馈**：[Issues 页面]
- **功能建议**：[Feature Requests]

## 🔄 版本历史

### v1.0.0 (当前版本)
- ✨ 初始版本发布
- ✅ 种子文件解析功能
- ✅ 现代化用户界面
- ✅ 安装器制作系统
- ✅ 多语言支持
- ✅ 使用条款保护

---

<div align="center">

**🌟 SeedParser - 让种子解析更简单 🌟**

*使用现代技术栈构建的强大种子解析工具*

</div>