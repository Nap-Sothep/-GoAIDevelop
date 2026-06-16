# PowerShell 启动脚本 - Windows环境
# 用途: 启动Docker Compose中间件并验证配置

param(
    [switch]$WithUI,  # 是否启动Kafka UI
    [switch]$SkipVerify  # 跳过验证
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Go Gateway - 启动Docker中间件服务" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 检查Docker是否安装
Write-Host "[1/5] 检查Docker环境..." -ForegroundColor Yellow
try {
    docker --version | Out-Null
    docker compose version | Out-Null
    Write-Host "  ✓ Docker已安装" -ForegroundColor Green
} catch {
    Write-Host "  ✗ Docker未安装或Docker未启动" -ForegroundColor Red
    Write-Host "  请先安装Docker Desktop: https://www.docker.com/products/docker-desktop" -ForegroundColor Yellow
    exit 1
}

# 检查Docker是否运行
try {
    docker info | Out-Null
    Write-Host "  ✓ Docker正在运行" -ForegroundColor Green
} catch {
    Write-Host "  ✗ Docker未运行，请启动Docker Desktop" -ForegroundColor Red
    exit 1
}

Write-Host ""

# 停止旧容器
Write-Host "[2/5] 停止旧的容器（如果存在）..." -ForegroundColor Yellow
docker compose down --remove-orphans 2>&1 | Out-Null
Write-Host "  ✓ 清理完成" -ForegroundColor Green

Write-Host ""

# 启动服务
Write-Host "[3/5] 启动中间件服务..." -ForegroundColor Yellow
if ($WithUI) {
    Write-Host "  启动模式: 包含Kafka UI (http://localhost:8081)" -ForegroundColor Cyan
    docker compose --profile ui up -d
} else {
    Write-Host "  启动模式: 仅中间件" -ForegroundColor Cyan
    docker compose up -d
}

Write-Host ""

# 等待服务就绪
Write-Host "[4/5] 等待服务就绪..." -ForegroundColor Yellow

function Wait-ForService {
    param($serviceName, $maxRetries = 30)
    $retry = 0
    while ($retry -lt $maxRetries) {
        $status = docker compose ps --format json | ConvertFrom-Json | Where-Object { $_.Service -eq $serviceName }
        if ($status -and $status.State -eq "running") {
            # 额外等待健康检查
            Start-Sleep -Seconds 5
            Write-Host "  ✓ $serviceName 已就绪" -ForegroundColor Green
            return $true
        }
        $retry++
        Write-Host "  等待 $serviceName... ($retry/$maxRetries)" -ForegroundColor Gray
        Start-Sleep -Seconds 2
    }
    Write-Host "  ✗ $serviceName 启动超时" -ForegroundColor Red
    return $false
}

Wait-ForService "mongodb"
Wait-ForService "redis"
Wait-ForService "zookeeper"
Wait-ForService "kafka"

Write-Host ""

# 验证连接
if (-not $SkipVerify) {
    Write-Host "[5/5] 验证中间件连接..." -ForegroundColor Yellow

    # 验证MongoDB
    try {
        docker exec go_gateway_mongodb mongosh --eval "db.adminCommand('ping')" --quiet | Out-Null
        Write-Host "  ✓ MongoDB (localhost:27017) - 连接成功" -ForegroundColor Green
    } catch {
        Write-Host "  ✗ MongoDB 连接失败" -ForegroundColor Red
    }

    # 验证Redis
    try {
        $result = docker exec go_gateway_redis redis-cli ping
        if ($result -eq "PONG") {
            Write-Host "  ✓ Redis (localhost:6379) - 连接成功" -ForegroundColor Green
        }
    } catch {
        Write-Host "  ✗ Redis 连接失败" -ForegroundColor Red
    }

    # 验证Kafka
    try {
        docker exec go_gateway_kafka kafka-topics.sh --list --bootstrap-server localhost:9092 | Out-Null
        Write-Host "  ✓ Kafka (localhost:9092) - 连接成功" -ForegroundColor Green
    } catch {
        Write-Host "  ✗ Kafka 连接失败" -ForegroundColor Red
    }

    Write-Host ""
}

# 显示服务状态
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  服务启动完成" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "中间件地址:" -ForegroundColor Yellow
Write-Host "  MongoDB:  mongodb://localhost:27017" -ForegroundColor White
Write-Host "  Redis:    redis://localhost:6379" -ForegroundColor White
Write-Host "  Kafka:    localhost:9092" -ForegroundColor White
Write-Host "  Zookeeper: localhost:2181" -ForegroundColor White

if ($WithUI) {
    Write-Host ""
    Write-Host "管理界面:" -ForegroundColor Yellow
    Write-Host "  Kafka UI: http://localhost:8081" -ForegroundColor Cyan
}

Write-Host ""
Write-Host "下一步操作:" -ForegroundColor Yellow
Write-Host "  1. 查看日志: docker compose logs -f" -ForegroundColor White
Write-Host "  2. 启动后端: go run cmd/server/main.go" -ForegroundColor White
Write-Host "  3. 启动网关: go run cmd/gateway/main.go" -ForegroundColor White
Write-Host "  4. 停止服务: docker compose down" -ForegroundColor White
Write-Host ""
Write-Host "提示: 使用 configs/config.docker.yaml 配置文件在Docker环境中运行应用" -ForegroundColor Cyan
Write-Host ""
