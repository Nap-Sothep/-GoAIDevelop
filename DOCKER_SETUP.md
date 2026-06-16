# Docker 环境部署指南

本文档说明如何使用 Docker Compose 部署项目依赖的中间件服务。

## 目录结构

```
GoAIDevelop/
├── docker-compose.yml          # Docker Compose配置文件
├── configs/
│   ├── config.yaml             # 本地开发配置（localhost）
│   └── config.docker.yaml      # Docker环境配置（服务名）
└── scripts/
    ├── start-docker.ps1        # 启动脚本（Windows PowerShell）
    └── verify-config.ps1       # 配置验证脚本
```

## 前置要求

- Docker Desktop 已安装并运行
- Docker Compose v2.0+ （内置于Docker Desktop）

## 快速开始

### 方式1: 使用启动脚本（推荐）

```powershell
# 进入项目目录
cd f:\MyProject\ai\GoAIDevelop

# 执行启动脚本
.\scripts\start-docker.ps1
```

**可选参数:**
```powershell
# 启动时包含Kafka UI管理界面
.\scripts\start-docker.ps1 -WithUI

# 跳过连接验证
.\scripts\start-docker.ps1 -SkipVerify
```

### 方式2: 手动启动

```powershell
# 1. 启动所有中间件
docker compose up -d

# 2. 查看服务状态
docker compose ps

# 3. 查看日志
docker compose logs -f

# 4. （可选）启动Kafka UI
docker compose --profile ui up -d
```

## 配置说明

### 本地开发模式 (config.yaml)

**适用场景**: 中间件在Docker中运行，应用在本地运行

```yaml
mongodb:
  uri: "mongodb://localhost:27017"  # Docker暴露端口到localhost

redis:
  addr: "localhost:6379"            # Docker暴露端口到localhost

kafka:
  brokers:
    - "localhost:9092"              # Docker暴露端口到localhost
```

**启动应用:**
```powershell
# 直接运行（使用config.yaml）
go run cmd/server/main.go
```

### Docker网络模式 (config.docker.yaml)

**适用场景**: 应用也在Docker容器中运行

```yaml
mongodb:
  uri: "mongodb://mongodb:27017"    # 使用Docker服务名

redis:
  addr: "redis:6379"                # 使用Docker服务名

kafka:
  brokers:
    - "kafka:29092"                 # 使用Docker内部地址
```

**启动应用:**
```powershell
# 设置环境变量指定配置文件
$env:GATEWAY_CONFIG_PATH='configs/config.docker.yaml'
go run cmd/server/main.go
```

## 中间件地址对照表

| 中间件 | 本地访问地址 | Docker内部地址 | 端口说明 |
|--------|-------------|---------------|---------|
| MongoDB | `localhost:27017` | `mongodb:27017` | 默认MongoDB端口 |
| Redis | `localhost:6379` | `redis:6379` | 默认Redis端口 |
| Kafka | `localhost:9092` | `kafka:29092` | 外部:9092, 内部:29092 |
| Zookeeper | `localhost:2181` | `zookeeper:2181` | Kafka依赖 |
| Kafka UI | `http://localhost:8081` | `kafka-ui:8080` | 可选管理界面 |

## 验证配置

### 使用验证脚本

```powershell
.\scripts\verify-config.ps1
```

脚本会检查：
1. 配置文件中的地址是否正确
2. 本地开发和Docker环境的配置差异
3. 端口占用情况

### 手动验证

```powershell
# 测试MongoDB连接
docker exec go_gateway_mongodb mongosh --eval "db.adminCommand('ping')"

# 测试Redis连接
docker exec go_gateway_redis redis-cli ping

# 测试Kafka连接
docker exec go_gateway_kafka kafka-topics.sh --list --bootstrap-server localhost:9092

# 查看所有容器状态
docker compose ps
```

## 常见问题

### 1. 端口被占用

**错误**: `Port 27017 is already in use`

**解决方案**:
```powershell
# 查找占用端口的进程
netstat -ano | findstr :27017

# 停止占用进程，或修改docker-compose.yml的端口映射
# 例如: - "27018:27017" 将外部端口改为27018
```

### 2. Kafka启动失败

**错误**: `Kafka broker failed to start`

**原因**: Zookeeper未就绪或端口冲突

**解决方案**:
```powershell
# 查看Kafka日志
docker compose logs kafka

# 重启Kafka
docker compose restart kafka

# 确保Zookeeper健康
docker compose ps zookeeper
```

### 3. 应用无法连接中间件

**检查清单**:
1. 确认Docker容器正在运行: `docker compose ps`
2. 确认配置文件使用正确的地址（localhost vs 服务名）
3. 检查防火墙是否阻止端口访问
4. 查看应用日志中的连接错误信息

### 4. 数据持久化

Docker Compose已配置卷（volumes），数据会持久化：

```powershell
# 查看卷
docker volume ls | findstr go_gateway

# 备份数据
docker run --rm -v go_gateway_mongodb_data:/data -v ${PWD}:/backup mongo tar czf /backup/mongodb-backup.tar.gz /data/db

# 恢复数据
docker run --rm -v go_gateway_mongodb_data:/data -v ${PWD}:/backup mongo bash -c "cd /data && tar xzf /backup/mongodb-backup.tar.gz"
```

## 停止服务

```powershell
# 停止所有服务（保留数据）
docker compose down

# 停止并删除所有数据（危险操作！）
docker compose down -v

# 停止特定服务
docker compose stop kafka
```

## 资源限制

如需限制容器资源使用，在 `docker-compose.yml` 中添加：

```yaml
services:
  mongodb:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          memory: 256M

  redis:
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
```

## 生产环境建议

1. **不要暴露端口**: 移除 `ports` 配置，仅通过Docker网络访问
2. **启用认证**: 为MongoDB和Redis设置密码
3. **使用Secrets**: 敏感信息通过Docker Secrets管理
4. **健康检查**: 启用healthcheck确保服务可用
5. **日志轮转**: 配置logging driver防止日志过大

示例（生产环境docker-compose.yml）:

```yaml
version: '3.8'

services:
  mongodb:
    image: mongo:7.0
    environment:
      MONGO_INITDB_ROOT_USERNAME: admin
      MONGO_INITDB_ROOT_PASSWORD: ${MONGODB_PASSWORD}
    ports: []  # 不暴露端口
    networks:
      - backend_network  # 仅内部网络

networks:
  backend_network:
    internal: true  # 内部网络，外部不可访问
```

## 监控和管理

### Kafka UI

启动后访问: http://localhost:8081

功能：
- 查看Topic列表和消息
- 创建/删除Topic
- 查看Consumer Group状态
- 发送测试消息

### MongoDB Compass

图形化管理工具: https://www.mongodb.com/products/compass

连接字符串: `mongodb://localhost:27017`

### Redis Insight

图形化管理工具: https://redis.com/redis-enterprise/redis-insight/

连接地址: `localhost:6379`

## 下一步

中间件启动后，继续：

1. **启动gRPC后端服务**:
   ```powershell
   go run cmd/server/main.go
   ```

2. **启动Gateway服务**:
   ```powershell
   go run cmd/gateway/main.go
   ```

3. **测试健康检查**:
   ```bash
   grpcurl -plaintext localhost:50051 hello.v1.HelloService/HealthCheck
   ```

4. **查看完整文档**: [README.md](../README.md)

## 技术支持

遇到问题请检查：
1. Docker Desktop版本是否最新
2. 系统资源是否充足（内存至少4GB可用）
3. 防火墙/杀毒软件是否阻止Docker
4. 查看详细日志: `docker compose logs`

常见错误和解决方案请参考项目主README的"常见问题"章节。
