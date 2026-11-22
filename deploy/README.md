# Windows 本地部署

## 环境准备

- Go 1.25 或更新版本，使用 `go version` 检查。
- Node.js 22.12 或更新版本，使用 `node -v` 和 `npm.cmd -v` 检查。
- Windows PowerShell 5.1 或 PowerShell 7。
- 已验证 Windows 版 Nginx 1.30.7。便携版可以解压到项目目录，无需修改系统 PATH。

首次在新机器部署时，从项目根目录下载并解压 Nginx。

```powershell
$runtime = Join-Path $PWD 'deploy/.runtime'
New-Item -ItemType Directory -Force -Path $runtime | Out-Null
$zip = Join-Path $runtime 'nginx-1.30.7.zip'
Invoke-WebRequest -UseBasicParsing 'https://nginx.org/download/nginx-1.30.7.zip' -OutFile $zip
Expand-Archive -LiteralPath $zip -DestinationPath $runtime
```

解压后应能看到 `deploy/.runtime/nginx-1.30.7/nginx.exe`。

## 构建与启停

```powershell
.\deploy\start.ps1
.\deploy\stop.ps1
```

启动脚本依次执行前端 `npm ci` 和 `npm run build`、后端 `go build`、`nginx -t`。Gin 成功初始化数据目录并通过健康检查后，脚本启动 Nginx，再从 Nginx 测一次健康接口。

参数可以调整入口、后端端口与数据目录。

```powershell
.\deploy\start.ps1 -WebPort 8188 -BackendPort 8180
.\deploy\start.ps1 -SkipBuild
```

运行配置由 `nginx.conf` 模板生成，保存在 `deploy/.runtime/nginx-1.30.7/conf/album.conf`。要修改分流或缓存规则，编辑模板后重启。默认 SQLite、图片和 JWT 密钥存放于 `backend/data`，可用 `-DataDirectory` 指定其他目录。清理运行产物时应保留需要备份的数据目录。

## 验证入口

```powershell
curl.exe -I http://127.0.0.1:8088/
curl.exe -i http://127.0.0.1:8088/api/v1/health
```

第一条返回 HTML 响应头，第二条返回包含 `data.status = ok` 的 JSON。两条响应的 `Server` 头均由 Nginx 设置。打开浏览器开发者工具 Network 面板，检查页面与 API 请求的协议、主机和端口一致。

- 后端日志位于 `deploy/.runtime/api.stdout.log` 和 `api.stderr.log`。
- Nginx 访问日志和错误日志位于便携版目录下的 `logs/`。
- 页面能打开而 API 返回 `502` 时，检查 Gin 运行状态和 `proxy_pass` 端口。
- 上传返回 `413` 时，检查照片 10 MB、头像 2 MB 的限制以及 Nginx 的 `client_max_body_size`。
- 默认两项服务均绑定到 `127.0.0.1`，用于本机演示。

## 发布到云服务器时

1. 构建前端静态文件与目标操作系统对应的 Go 可执行文件。
2. 将构建产物部署到服务器。使用服务管理器运行后端，指定持久化的数据目录和 `ALBUM_JWT_SECRET`。
3. 配置 Nginx 站点的域名、证书、静态根路径和反代地址，保留 `/api/` 与 `/uploads/` 前缀。
4. 通过 HTTPS 访问入口，检查健康、公开照片与私有照片授权。纳入数据备份和运行日志检查。
