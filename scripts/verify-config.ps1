# PowerShell 验证脚本 - 检查配置一致性
# 用途: 验证config.yaml与Docker服务地址是否匹配

$ErrorActionPreference = "Continue"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Go Gateway - 配置一致性检查" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$configFile = "configs/config.yaml"
$dockerConfigFile = "configs/config.docker.yaml"

# 读取配置文件
if (-not (Test-Path $configFile)) {
    Write-Host "✗ 配置文件不存在: $configFile" -ForegroundColor Red
    exit 1
}

Write-Host "读取配置文件: $configFile" -ForegroundColor Green
$configContent = Get-Content $configFile -Raw

Write-Host ""

# 提取配置项
function Extract-Config {
    param($content, $key)
    if ($content -match "$key`:.*[`"']?([^`"'\s]+)[`"']?") {
        return $matches[1]
    }
    return $null
}

# 检查MongoDB配置
Write-Host "[1/3] MongoDB配置检查" -ForegroundColor Yellow
$mongoUri = Extract-Config $configContent "uri"
if ($mongoUri -eq "mongodb://localhost:27017") {
    Write-Host "  ✓ MongoDB URI: $mongoUri" -ForegroundColor Green
    Write-Host "    → Docker环境应使用: mongodb://mongodb:27017" -ForegroundColor Cyan
} else {
    Write-Host "  ⚠ MongoDB URI: $mongoUri" -ForegroundColor Yellow
    Write-Host "    → 本地开发使用localhost，Docker使用mongodb服务名" -ForegroundColor Gray
}

Write-Host ""

# 检查Redis配置
Write-Host "[2/3] Redis配置检查" -ForegroundColor Yellow
$redisAddr = Extract-Config $configContent "addr"
if ($redisAddr -eq "localhost:6379") {
    Write-Host "  ✓ Redis地址: $redisAddr" -ForegroundColor Green
    Write-Host "    → Docker环境应使用: redis:6379" -ForegroundColor Cyan
} else {
    Write-Host "  ⚠ Redis地址: $redisAddr" -ForegroundColor Yellow
    Write-Host "    → 本地开发使用localhost，Docker使用redis服务名" -ForegroundColor Gray
}

Write-Host ""

# 检查Kafka配置
Write-Host "[3/3] Kafka配置检查" -ForegroundColor Yellow
if ($configContent -match 'brokers:[\s\S]*?- [`"']?([^`"'\s]+)[`"']?') {
    $kafkaBroker = $matches[1]
    if ($kafkaBroker -eq "localhost:9092") {
        Write-Host "  ✓ Kafka Broker: $kafkaBroker" -ForegroundColor Green
        Write-Host "    → Docker环境应使用: kafka:29092" -ForegroundColor Cyan
        Write-Host "    → 外部访问使用: localhost:9092" -ForegroundColor Cyan
    } else {
        Write-Host "  ⚠ Kafka Broker: $kafkaBroker" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  配置说明" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "本地开发 (config.yaml):" -ForegroundColor Yellow
Write-Host "  - 使用 localhost 作为所有中间件地址" -ForegroundColor White
Write-Host "  - 直接运行: go run cmd/server/main.go" -ForegroundColor White
Write-Host ""
Write-Host "Docker环境 (config.docker.yaml):" -ForegroundColor Yellow
Write-Host "  - 使用 Docker 服务名 (mongodb, redis, kafka)" -ForegroundColor White
Write-Host "  - 设置环境变量: `$env:GATEWAY_CONFIG_PATH='configs/config.docker.yaml'" -ForegroundColor White
Write-Host "  - 或在容器内运行应用" -ForegroundColor White
Write-Host ""
Write-Host "混合模式:" -ForegroundColor Yellow
Write-Host "  - 中间件在Docker中运行" -ForegroundColor White
Write-Host "  - 应用在本地运行" -ForegroundColor White
Write-Host "  - 使用 config.yaml (localhost) 即可" -ForegroundColor White
Write-Host ""

# 检查端口占用
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  端口占用检查" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$ports = @{
    "MongoDB" = 27017
    "Redis" = 6379
    "Kafka" = 9092
    "Zookeeper" = 2181
    "Kafka UI" = 8081
}

foreach ($name in $ports.Keys) {
    $port = $ports[$name]
    $connection = $null
    try {
        $connection = New-Object System.Net.Sockets.TcpClient("localhost", $port)
        Write-Host "✓ 端口 $port ($name) - 已占用" -ForegroundColor Green
    } catch {
        Write-Host "○ 端口 $port ($name) - 空闲" -ForegroundColor Gray
    } finally {
        if ($connection) { $connection.Close() }
    }
}

Write-Host ""
Write-Host "提示: 如果端口已被占用，请先停止占用进程或修改docker-compose.yml中的端口映射" -ForegroundColor Cyan
Write-Host ""
