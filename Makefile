.PHONY: all build run proto clean lint test

# 模块名
MODULE := go-gateway

# 构建输出
BINARY := bin/gateway

# Proto目录
PROTO_DIR := proto

# 默认目标
all: proto build

# 生成Proto Go代码
proto:
	@echo "生成Proto代码..."
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       $(PROTO_DIR)/hello/v1/hello.proto

# 编译
build:
	@echo "编译 $(BINARY)..."
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/gateway

# 运行
run:
	@echo "启动Gateway..."
	go run ./cmd/gateway

# 代码检查
lint:
	@echo "运行go vet..."
	go vet ./...
	@echo "运行golangci-lint..."
	golangci-lint run ./... || echo "golangci-lint 未安装，跳过"

# 测试
test:
	go test -v -race -cover ./...

# 清理
clean:
	@echo "清理构建产物..."
	rm -rf bin/

# 安装依赖
deps:
	go mod tidy
	go mod download
