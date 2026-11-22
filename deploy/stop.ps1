[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$runtime = Join-Path $PSScriptRoot '.runtime'
if (-not (Test-Path -LiteralPath $runtime)) { return }

# 独立的 pid 文件使停止命令只影响本项目的 Nginx 实例。
Get-ChildItem -LiteralPath $runtime -Directory -Filter 'nginx-*' | ForEach-Object {
    $nginx = Join-Path $_.FullName 'nginx.exe'
    $pidFile = Join-Path $_.FullName 'logs/nginx.pid'
    if ((Test-Path -LiteralPath $nginx) -and (Test-Path -LiteralPath $pidFile)) {
        $nginxId = Get-Content -LiteralPath $pidFile -TotalCount 1
        $process = Get-Process -Id $nginxId -ErrorAction SilentlyContinue
        if ($process -and $process.Path -eq $nginx) {
            $prefix = ($_.FullName -replace '\\', '/') + '/'
            Push-Location $_.FullName
            try {
                & $nginx -p $prefix -c 'conf/album.conf' -s quit
                if ($LASTEXITCODE -ne 0) { throw 'Nginx 停止失败。' }
            } finally { Pop-Location }
            if (-not $process.WaitForExit(5000)) { throw 'Nginx 正在退出，请稍后重试。' }
        }
    }
}

$stateFile = Join-Path $runtime 'backend.json'
if (Test-Path -LiteralPath $stateFile) {
    $state = Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json
    $process = Get-Process -Id $state.pid -ErrorAction SilentlyContinue
    $expected = Join-Path $runtime 'album-api.exe'
    # 校验可执行文件与启动时间，避免 PID 被复用时停止其他进程。
    if ($process -and $process.Path -eq $expected -and [string]$process.StartTime.ToUniversalTime().Ticks -eq $state.startTicks) {
        Stop-Process -Id $process.Id
        if (-not $process.WaitForExit(5000)) { throw '后端正在退出，请稍后重试。' }
    }
    Remove-Item -LiteralPath $stateFile
}
Write-Host '相册服务已停止。'
