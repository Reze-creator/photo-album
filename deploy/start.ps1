[CmdletBinding()]
param(
    [ValidateRange(1024, 65535)][int]$WebPort = 8088,
    [ValidateRange(1024, 65535)][int]$BackendPort = 8080,
    [string]$DataDirectory,
    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
$runtime = Join-Path $PSScriptRoot '.runtime'
$frontend = Join-Path $project 'frontend'
$backend = Join-Path $project 'backend'
$dist = Join-Path $frontend 'dist'
$backendExe = Join-Path $runtime 'album-api.exe'
$stateFile = Join-Path $runtime 'backend.json'
$nginxHome = Join-Path $runtime 'nginx-1.30.7'
$nginx = Join-Path $nginxHome 'nginx.exe'
$configName = 'conf/album.conf'
$prefix = ($nginxHome -replace '\\', '/') + '/'
$origin = "http://127.0.0.1:$WebPort"

function Wait-Health([string]$Origin) {
    for ($attempt = 0; $attempt -lt 20; $attempt++) {
        try {
            $health = Invoke-RestMethod -Uri "$Origin/api/v1/health" -TimeoutSec 2
            if ($health.code -eq 0 -and $health.data.status -eq 'ok') { return }
        } catch {
            # 服务可能正在启动，下一轮再次检查。
        }
        Start-Sleep -Milliseconds 200
    }
    throw "健康检查未通过。请检查 $Origin/api/v1/health 和运行日志。"
}

if ($WebPort -eq $BackendPort) { throw 'Nginx 和 Gin 需要不同的端口。' }
if (Test-Path -LiteralPath $stateFile) { throw '请先执行 deploy/stop.ps1 清理上次的运行状态。' }
if (Get-NetTCPConnection -LocalPort $WebPort,$BackendPort -State Listen -ErrorAction SilentlyContinue) {
    throw '端口已占用，请通过 -WebPort 和 -BackendPort 更换端口。'
}
if (-not (Test-Path -LiteralPath $nginx)) {
    throw '未找到 Windows 版 Nginx，请按 deploy/README.md 中的命令解压便携版。'
}
if (-not $DataDirectory) { $DataDirectory = Join-Path $backend 'data' }
$DataDirectory = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($DataDirectory)

if (-not $SkipBuild) {
    Push-Location $frontend
    try {
        npm.cmd ci --no-audit --no-fund
        if ($LASTEXITCODE -ne 0) { throw 'npm ci 失败。' }
        npm.cmd run build
        if ($LASTEXITCODE -ne 0) { throw '前端构建失败。' }
    } finally { Pop-Location }
    Push-Location $backend
    try {
        go build -trimpath -o $backendExe ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw '后端构建失败。' }
    } finally { Pop-Location }
}
if (-not (Test-Path -LiteralPath (Join-Path $dist 'index.html')) -or -not (Test-Path -LiteralPath $backendExe)) {
    throw '缺少构建产物，请去掉 -SkipBuild 后重新启动。'
}

# Windows 版 Nginx 使用正斜杠，运行配置使用无 BOM 的 UTF-8。
$template = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'nginx.conf') -Raw -Encoding UTF8
$config = $template.Replace('__FRONTEND_DIST__', ($dist -replace '\\', '/'))
$config = $config.Replace('__WEB_PORT__', "$WebPort").Replace('__BACKEND_PORT__', "$BackendPort")
[IO.File]::WriteAllText((Join-Path $nginxHome $configName), $config, [Text.UTF8Encoding]::new($false))
Push-Location $nginxHome
try {
    & $nginx -p $prefix -c $configName -t
    if ($LASTEXITCODE -ne 0) { throw 'Nginx 配置检查失败。' }
} finally { Pop-Location }

$previousAddr = $env:ALBUM_ADDR
$previousData = $env:ALBUM_DATA_DIR
try {
    $env:ALBUM_ADDR = "127.0.0.1:$BackendPort"
    $env:ALBUM_DATA_DIR = $DataDirectory
    $backendOptions = @{
        FilePath = $backendExe
        WorkingDirectory = $backend
        WindowStyle = 'Hidden'
        PassThru = $true
        RedirectStandardOutput = (Join-Path $runtime 'api.stdout.log')
        RedirectStandardError = (Join-Path $runtime 'api.stderr.log')
    }
    $process = Start-Process @backendOptions
    @{ pid = $process.Id; startTicks = [string]$process.StartTime.ToUniversalTime().Ticks } |
        ConvertTo-Json -Compress | Set-Content -LiteralPath $stateFile -Encoding ASCII
    Wait-Health "http://127.0.0.1:$BackendPort"

    $nginxArgs = '-p "{0}" -c {1}' -f $prefix, $configName
    Start-Process -FilePath $nginx -WorkingDirectory $nginxHome -WindowStyle Hidden -ArgumentList $nginxArgs | Out-Null
    Wait-Health $origin
} catch {
    & (Join-Path $PSScriptRoot 'stop.ps1')
    throw
} finally {
    $env:ALBUM_ADDR = $previousAddr
    $env:ALBUM_DATA_DIR = $previousData
}

Write-Host "相册已启动 $origin"
Write-Host "Nginx 健康检查已通过 $origin/api/v1/health"
