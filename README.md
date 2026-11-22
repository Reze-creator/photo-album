# 留影 · 个人相册

一个使用 Go、SQLite 和 Vue 实现的前后端分离相册。浏览器从 Nginx 加载页面，并在同一地址请求 API 和图片。

| 作业信息 | 内容 |
| --- | --- |
| 学校 | 待填写 |
| 姓名 | 待填写 |
| 学号 | 待填写 |

## 功能清单

- [x] 上传头像注册、登录、JWT 登录状态持久化
- [x] 创建公开或私有相册，分页查看自己的相册
- [x] 公开广场展示相册、创建者用户名和头像
- [x] 相册详情和照片分页，主人可上传照片，成功后刷新列表
- [x] 待加载、无数据和失败状态，受保护页面的登录跳转
- [x] 私有相册的详情、照片列表和原图访问控制
- [x] Nginx 同源入口、健康检查、Windows 构建与启停脚本

## 技术选型与目录

后端使用 Go 1.25+、Gin、SQLite（`modernc.org/sqlite`，无需 C 编译器）、bcrypt 和 JWT。前端使用 Vue 3、Vue Router 和 Vite。部署验证使用 Windows 版 Nginx 1.30.7。

```text
photo-album/
├─ backend/
│  ├─ cmd/server/        服务入口、运行配置和密钥管理
│  └─ internal/
│     ├─ api/            路由、鉴权、相册、上传及接口测试
│     └─ store/          SQLite 表结构和数据访问
├─ frontend/src/         页面、组件、同源 API 封装
├─ deploy/               Nginx 配置模板、启停脚本和部署说明
└─ docs/                 接口约定和验收步骤
```

```text
浏览器 → Nginx 127.0.0.1:8088
          ├─ /          → frontend/dist
          ├─ /api/      → Gin 127.0.0.1:8080 → SQLite
          └─ /uploads/  → Gin 检查权限后读取图片文件
```

## 在 Windows 启动

在项目根目录打开 PowerShell。首次使用时按 [部署说明](deploy/README.md) 准备 Windows 版 Nginx。本机项目目录已备好该工具。

```powershell
.\deploy\start.ps1
```

脚本构建前端和后端，检查 Nginx 配置并等待健康检查通过。访问 `http://127.0.0.1:8088`。停止时执行 `./deploy/stop.ps1`。已构建时可用 `./deploy/start.ps1 -SkipBuild`。

开发时分别启动 `backend` 下的 `go run ./cmd/server` 和 `frontend` 下的 `npm.cmd ci`、`npm.cmd run dev`。Vite 代理 `/api/` 与 `/uploads/`，部署验收使用 Nginx 入口。

## 实现细节

- **图片访问权限**。图片 URL 通过 Nginx 转发给 Gin。Gin 查到图片所属相册后复用读权限判断。Vue 使用携带 JWT 的请求取得私有图片，再用临时 Blob URL 渲染。
- **上传与数据库一致性**。后端从图片内容识别格式，并限制文件大小和像素尺寸。文件用随机名称排他创建。数据库写入失败时清理刚保存的文件。
- **服务重启后的登录**。JWT 有效期为 24 小时。默认签名密钥保存在数据目录，继续使用同一目录或签名环境变量即可在重启后验证之前签发的 Token。
- **部署路径**。Nginx 把 `/api/` 原样转给 Gin。只在页面路由走 SPA 回退，静态资源或接口不存在时保留各自的错误响应。

## 接口与验收

JSON 响应为 `{ "code": 0, "message": "ok", "data": ... }`。失败时 `code` 取对应的 HTTP 状态码，`data` 为 `null`。分页结果包含 `items`、`total`、`page` 和 `page_size`。完整说明见 [接口文档](docs/api.md)。

在 `backend` 目录执行 `go test ./...`。接口测试覆盖注册、头像可读、登录、公开和私有相册可见性、分页、上传格式验证、同名文件防覆盖和跨用户权限。浏览器验收及录屏顺序见 [验收清单](docs/acceptance.md)。
