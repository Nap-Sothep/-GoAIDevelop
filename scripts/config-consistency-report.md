# 配置一致性检查报告

生成时间: 2026-06-11
检查范围: configs/config.yaml vs docker-compose.yml

---

## ✅ 配置一致性验证结果

### 1. MongoDB 配置

| 配置项 | config.yaml | docker-compose.yml | 状态 |
|--------|-------------|-------------------|------|
| 主机地址 | localhost | mongodb (服务名) | ✅ 正确 |
| 端口 | 27017 | 27017 (映射到localhost:27017) | ✅ 一致 |
| 数据库 | go_gateway_db | MONGO_INITDB_DATABASE=go_gateway_db | ✅ 一致 |
| 连接字符串 | `mongodb://localhost:27017` | `mongodb://mongodb:27017` (Docker环境) | ✅ 正确 |

**说明**:
- ✅ 本地开发使用 `localhost:27017` - 正确
- ✅ Docker环境使用 `mongodb:27017` - 正确（config.docker.yaml已配置）
- ✅ 端口映射 `27017:27017` 符合预期

---

### 2. Redis 配置

| 配置项 | config.yaml | docker-compose.yml | 状态 |
|--------|-------------|-------------------|------|
| 主机地址 | localhost | redis (服务名) | ✅ 正确 |
| 端口 | 6379 | 6379 (映射到localhost:6379) | ✅ 一致 |
| 密码 | "" (空) | 未设置（默认无密码） | ✅ 一致 |
| 连接地址 | `localhost:6379` | `redis:6379` (Docker环境) | ✅ 正确 |

**说明**:
- ✅ 本地开发使用 `localhost:6379` - 正确
- ✅ Docker环境使用 `redis:6379` - 正确（config.docker.yaml已配置）
- ✅ 端口映射 `6379:6379` 符合预期

---

### 3. Kafka 配置

| 配置项 | config.yaml | docker-compose.yml | 状态 |
|--------|-------------|-------------------|------|
| Broker地址 | localhost | kafka (服务名) | ✅ 正确 |
| 外部端口 | 9092 | 9092 (映射到localhost:9092) | ✅ 一致 |
| 内部端口 | - | 29092 (容器间通信) | ✅ 正确 |
| Advertised Listeners | - | PLAINTEXT_HOST://localhost:9092 | ✅ 正确 |
| 连接地址 | `localhost:9092` | `kafka:29092` (Docker环境) | ✅ 正确 |

**关键配置说明**:

docker-compose.yml中的Kafka监听器配置：
```yaml
KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:29092,PLAINTEXT_HOST://localhost:9092
```

这意味着：
- ✅ **容器间通信**: 使用 `kafka:29092`
- ✅ **外部访问**: 使用 `localhost:9092`
- ✅ config.yaml使用 `localhost:9092` - 正确（外部访问）
- ✅ config.docker.yaml使用 `kafka:29092` - 正确（容器内访问）

---

### 4. Zookeeper 配置

| 配置项 | docker-compose.yml | 状态 |
|--------|-------------------|------|
| 服务名 | zookeeper | ✅ |
| 端口 | 2181 | ✅ |
| Kafka依赖 | depends_on: zookeeper | ✅ |

**说明**:
- ✅ Zookeeper作为Kafka的依赖已正确配置
- ✅ 端口2181已暴露供外部调试使用

---

## 📋 配置文件对比总结

### config.yaml (本地开发)

```yaml
mongodb:
  uri: "mongodb://localhost:27017"    # ✓ 用于访问Docker暴露的端口

redis:
  addr: "localhost:6379"              # ✓ 用于访问Docker暴露的端口

kafka:
  brokers:
    - "localhost:9092"                # ✓ 用于访问Docker暴露的端口
```

**适用场景**: 
- 中间件在Docker中运行
- 应用在本地运行（go run）
- 通过Docker端口映射访问中间件

---

### config.docker.yaml (Docker环境)

```yaml
mongodb:
  uri: "mongodb://mongodb:27017"      # ✓ 使用Docker服务名

redis:
  addr: "redis:6379"                  # ✓ 使用Docker服务名

kafka:
  brokers:
    - "kafka:29092"                   # ✓ 使用Docker内部地址
```

**适用场景**:
- 应用和中间件都在Docker网络中运行
- 通过Docker DNS解析服务名
- 不经过端口映射，直接容器间通信

---

## ⚠️ 注意事项

### 1. Kafka双监听器配置

**重要**: Kafka配置了两个监听器，这是正确的做法：

```yaml
KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:29092,PLAINTEXT_HOST://localhost:9092
```

**原因**:
- 容器内的客户端（如Kafka UI）使用 `kafka:29092`
- 宿主机上的应用使用 `localhost:9092`
- 避免路由问题

---

### 2. 环境变量覆盖

项目支持通过环境变量覆盖配置：

```powershell
# 覆盖MongoDB地址
$env:GATEWAY_MONGODB_URI="mongodb://custom-host:27017"

# 覆盖Redis地址
$env:GATEWAY_REDIS_ADDR="custom-redis:6379"

# 覆盖Kafka Brokers
$env:GATEWAY_KAFKA_BROKERS_0="custom-kafka:9092"
```

**优先级**: 环境变量 > config.yaml文件

---

### 3. 健康检查

docker-compose.yml已为所有服务配置健康检查：

| 服务 | 健康检查命令 | 间隔 | 超时 | 重试 |
|------|------------|------|------|------|
| MongoDB | `mongosh --eval "db.adminCommand('ping')"` | 10s | 5s | 5次 |
| Redis | `redis-cli ping` | 10s | 5s | 5次 |
| Zookeeper | `nc -z localhost 2181` | 10s | 5s | 5次 |
| Kafka | `kafka-broker-api-versions --bootstrap-server localhost:9092` | 15s | 10s | 5次 |

**启动脚本会等待所有服务健康后再继续。**

---

## 🚀 启动验证步骤

### 步骤1: 启动Docker中间件

```powershell
cd f:\MyProject\ai\GoAIDevelop

# 方式A: 使用启动脚本（推荐）
.\scripts\start-docker.ps1

# 方式B: 手动启动
docker compose up -d
```

### 步骤2: 验证服务状态

```powershell
# 查看所有容器状态
docker compose ps

# 预期输出：所有容器状态为 "Up (healthy)"
NAME                     STATUS
go_gateway_mongodb       Up (healthy)
go_gateway_redis         Up (healthy)
go_gateway_zookeeper     Up (healthy)
go_gateway_kafka         Up (healthy)
```

### 步骤3: 运行配置验证脚本

```powershell
.\scripts\verify-config.ps1
```

脚本会检查：
- ✅ 配置文件地址是否正确
- ✅ 端口占用情况
- ✅ 本地vs Docker配置差异

### 步骤4: 测试中间件连接

```powershell
# 测试MongoDB
docker exec go_gateway_mongodb mongosh --eval "db.adminCommand('ping')"
# 预期输出: { ok: 1 }

# 测试Redis
docker exec go_gateway_redis redis-cli ping
# 预期输出: PONG

# 测试Kafka
docker exec go_gateway_kafka kafka-topics.sh --list --bootstrap-server localhost:9092
# 预期输出: (空列表，表示连接成功)
```

### 步骤5: 启动应用并测试

```powershell
# 启动gRPC后端
go run cmd/server/main.go

# 新终端启动Gateway
go run cmd/gateway/main.go

# 测试健康检查
grpcurl -plaintext localhost:50051 hello.v1.HelloService/HealthCheck
```

**预期响应**:
```json
{
  "status": "SERVING",
  "mongodb": {"available": true, "message": "正常"},
  "redis": {"available": true, "message": "正常"},
  "kafka": {"available": true, "message": "正常"}
}
```

---

## 🔧 故障排查

### 问题1: 配置不一致

**症状**: 应用无法连接中间件

**检查**:
```powershell
# 确认使用的配置文件
echo $env:GATEWAY_CONFIG_PATH

# 查看实际配置
Get-Content configs/config.yaml | Select-String "localhost:"
```

**解决**: 确保config.yaml使用localhost，config.docker.yaml使用服务名

---

### 问题2: 端口冲突

**症状**: Docker容器启动失败，提示端口被占用

**检查**:
```powershell
netstat -ano | findstr :27017
netstat -ano | findstr :6379
netstat -ano | findstr :9092
```

**解决**:
```powershell
# 方案A: 停止占用进程
Stop-Process -Id <PID>

# 方案B: 修改docker-compose.yml端口映射
# 例如: - "27018:27017" 将外部端口改为27018
```

---

### 问题3: Kafka无法连接

**症状**: 应用日志显示 "Kafka connection refused"

**检查**:
```powershell
# 查看Kafka日志
docker compose logs kafka | Select-String "ERROR"

# 确认监听器配置
docker exec go_gateway_kafka cat /etc/kafka/kafka.properties | Select-String "advertised.listeners"
```

**预期输出**:
```
advertised.listeners=PLAINTEXT://kafka:29092,PLAINTEXT_HOST://localhost:9092
```

**解决**: 如果监听器配置不正确，重启Kafka容器
```powershell
docker compose restart kafka
```

---

## ✅ 结论

### 配置一致性评分: 100/100

| 检查项 | 得分 | 说明 |
|-------|------|------|
| MongoDB配置 | ✅ 25/25 | 地址、端口、数据库名称完全一致 |
| Redis配置 | ✅ 25/25 | 地址、端口、密码配置正确 |
| Kafka配置 | ✅ 25/25 | 双监听器配置正确，内外地址分离 |
| Docker网络 | ✅ 25/25 | 服务名、端口映射、DNS解析正确 |

### 最终评估

✅ **配置完全正确，可以启动**

**理由**:
1. ✅ config.yaml使用localhost地址 - 适合本地开发访问Docker暴露的端口
2. ✅ config.docker.yaml使用Docker服务名 - 适合容器内应用访问
3. ✅ docker-compose.yml端口映射正确 - 所有中间件端口已正确暴露
4. ✅ Kafka双监听器配置正确 - 解决了容器内外访问路由问题
5. ✅ 健康检查已配置 - 确保服务就绪后才接受请求
6. ✅ 数据持久化已配置 - volumes确保数据不丢失

### 推荐启动流程

```powershell
# 1. 启动中间件
.\scripts\start-docker.ps1

# 2. 验证配置
.\scripts\verify-config.ps1

# 3. 启动后端
go run cmd/server/main.go

# 4. （新终端）启动网关
go run cmd/gateway/main.go

# 5. 测试
grpcurl -plaintext localhost:50051 hello.v1.HelloService/HealthCheck
```

**预期结果**: 所有中间件连接成功，HealthCheck返回SERVING状态。

---

## 📚 相关文档

- [Docker部署指南](../DOCKER_SETUP.md)
- [项目README](../README.md)
- [配置说明](../configs/config.yaml)
