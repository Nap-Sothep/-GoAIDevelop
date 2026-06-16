---
title: "feat: 需求管理系统全栈实现"
type: feat
date: 2026-06-16
origin: docs/brainstorms/2026-06-16-requirement-management-requirements.md
---

## Summary

构建需求管理系统的全栈实现，包含 Go 后端（Gin REST API + gRPC Service + MongoDB）和 React 前端（Vite + TypeScript + Ant Design）。系统支持需求的查看、创建、编辑、详情展示，以及 Markdown 文档导入和需求关联功能。经验管理仅提供 API 接口，不在 UI 中操作。

## Problem Frame

项目已有需求管理的 gRPC proto 定义（`api/requirement/v1/requirement.proto`），但尚未实现具体的业务逻辑和前端界面。需要基于现有的 Go 微服务脚手架架构（Gin + gRPC + MongoDB + Redis + Kafka）快速落地一个可运行的需求管理系统，用于支撑 Compound Engineering 工作流中的需求追踪和经验沉淀。

## Requirements

**核心需求：**

R1. 需求列表页：分页展示所有需求，按创建时间倒序排列，基础列表无需筛选和搜索。

R2. 创建需求：表单包含标题、描述字段，支持上传 Markdown 文件自动提取内容填充到描述。

R3. 编辑需求：可修改标题、描述，支持关联其他需求（简单双向关联，不区分关系类型）。

R4. 需求详情页：展示需求完整信息（标题、描述、创建/更新时间）、关联需求列表、关联经验列表（只读展示）。

R5. 经验 API：提供新增经验的 gRPC 和 HTTP 接口，无 UI 操作入口，接口供后续扩展使用。

R6. 数据存储：使用 MongoDB 持久化需求和经验数据，遵循项目现有的 Repository 模式。

R7. API 设计：同时提供 gRPC Service（proto 已定义）和 HTTP REST API（供前端调用），Gin Handler 作为适配层。

R8. 无用户认证：简化版实现，所有用户共享同一个需求池，无需权限控制。

## Key Technical Decisions

**KTD1: 单体应用架构（方案 A）**

决策：Go 后端同时提供 gRPC 服务和 HTTP REST API，React 前端作为独立 SPA 由 Gin 托管静态文件。

理由：与项目现有的 Gin + gRPC 集成架构一致，部署简单（单一二进制文件），前后端同仓库便于协作。未来如需拆分为微服务，gRPC 服务已定义好可直接提取。

风险：需维护两套 API（gRPC + REST），但 Gin Handler 层很薄，主要是参数转换和调用 Service 层。

**KTD2: 最小字段集 + 完整功能**

决策：需求实体仅使用核心字段（id, title, description, related_ids, created_at, updated_at），但实现所有功能（包括 Markdown 导入、需求关联、经验展示）。

理由：用户明确要求最小字段集，避免过度设计。priority、status、tags 等 proto 中定义的字段暂不使用，保持数据结构简洁。

影响：proto 中的部分字段（priority, status, tags, generated_files 等）在实现中保留但不使用，为后续扩展留空间。

**KTD3: MongoDB 文档结构设计**

决策：需求集合名为 `requirements`，经验集合名为 `experiences`，使用 BSON ObjectId 作为主键。需求关联通过 `related_ids` 数组字段存储关联需求的 ID 列表。

理由：遵循项目现有的 MongoDB 使用模式（参考 UserRepository），文档结构扁平化便于查询和索引。关联关系存储在数组中，查询时通过 `$in` 操作符批量获取关联需求。

技术细节：
- 需求模型：`Requirement { ID, Title, Description, RelatedIDs []string, CreatedAt, UpdatedAt }`
- 经验模型：沿用 proto 定义的 Experience 结构（完整字段）
- 索引：`created_at: -1`（列表排序），`related_ids: 1`（关联查询优化）

**KTD4: React + Vite + Ant Design 技术选型**

决策：前端使用 Vite 构建工具，React 18 + TypeScript，Ant Design 5.x 组件库。

理由：Ant Design 是企业级 React 组件库，开箱即用，适合快速构建管理后台类应用。Vite 提供快速的开发体验和高效的构建性能。TypeScript 保证类型安全，与后端的 proto 类型定义对应。

依赖管理：前端代码放在项目根目录的 `web/` 子目录，与 `internal/` 平级，独立的 `package.json` 和 `node_modules`。

**KTD5: Markdown 文档导入实现方式**

决策：前端读取 Markdown 文件内容为纯文本字符串，直接赋值给描述的初始值，用户可在提交前编辑。不进行复杂的 Markdown 解析或结构化提取。

理由：用户要求"上传即提取"，但未要求智能解析。纯文本提取实现简单，避免引入额外的 Markdown 解析库。后端只需接收字符串字段，无需处理文件二进制。

流程：前端 `<input type="file">` 读取 `.md` 文件 → FileReader 读取文本内容 → 填充到描述 textarea → 用户可编辑 → 提交时发送纯文本。

**KTD6: 需求关联的数据流**

决策：关联操作通过单独的 API 端点实现（`POST /api/v1/requirements/:id/link`），接收目标需求 ID。后端将目标 ID 添加到 `related_ids` 数组，同时将当前需求 ID 也添加到目标需求的 `related_ids` 数组（双向关联）。

理由：简单双向关联不需要关系类型字段，实现成本低。两次更新操作在一个事务中完成（MongoDB BulkWrite），保证数据一致性。

注意：需防止自关联（不能关联自己）和重复关联（同一需求不能添加多次）。

## Implementation Units

### U1. MongoDB Repository 层实现

**Goal:** 实现需求和经验的数据访问层，遵循项目现有的 Repository 模式。

**Requirements:** R6

**Dependencies:** 无

**Files:**
- `internal/model/requirement.go`（新建）
- `internal/model/experience.go`（新建）
- `internal/repository/requirement_repo.go`（新建）
- `internal/repository/experience_repo.go`（新建）

**Approach:**

参考 `internal/model/user.go` 和 `internal/repository/user_repo.go` 的模式：

1. 创建 Requirement 和 Experience 模型结构体，定义 BSON 标签和 JSON 标签
2. 实现 CollectionName() 方法返回集合名
3. 创建 RequirementRepository 和 ExperienceRepository 结构体，持有 `*mongo.Collection`
4. 实现 CRUD 方法：Create, GetByID, Update, Delete, List（分页）
5. 实现关联操作方法：LinkRequirement（双向关联）, GetRelatedRequirements
6. 在 NewXXXRepository 构造函数中自动创建必要的 MongoDB 索引

关键代码模式：
- 使用 `bson.ObjectID` 作为 ID 类型
- 使用 `time.Time` 记录创建和更新时间
- 使用 `mongo.ErrNoDocuments` 判断记录不存在
- 使用 `BulkWrite` 实现原子性的双向关联更新
- 索引：`created_at: -1`, `related_ids: 1`

**Test scenarios:**

Covers R6. 测试场景：
- 创建需求后验证 MongoDB 中有对应文档，ID 已回填
- 根据 ID 查询存在的需求，返回正确数据
- 根据 ID 查询不存在的需求，返回 (nil, nil)
- 更新需求后验证字段已变更，updated_at 已刷新
- 删除需求后验证文档已从数据库移除
- 分页查询返回列表，按 created_at 倒序，总数正确
- 双向关联：关联 A→B 后，A 的 related_ids 包含 B，B 的 related_ids 包含 A
- 防止自关联：尝试关联自己时返回错误
- 防止重复关联：重复关联同一需求时幂等处理（不报错但不重复添加）

**Verification:** 运行单元测试，验证所有 CRUD 和关联操作符合预期，MongoDB 索引已创建。

---

### U2. gRPC Service 层实现

**Goal:** 实现 proto 定义的 RequirementService 和 ExperienceService，编排 Repository 层完成业务逻辑。

**Requirements:** R7

**Dependencies:** U1

**Files:**
- `internal/service/requirement_service.go`（新建）
- `internal/service/experience_service.go`（新建）

**Approach:**

参考 `internal/service/user_service.go` 的模式：

1. 创建 RequirementService 结构体，注入 `*repository.RequirementRepository`
2. 实现 CreateRequirement, GetRequirement, UpdateRequirement, DeleteRequirement, ListRequirements 方法
3. 实现 LinkRequirements, GetRelatedRequirements 方法
4. 创建 ExperienceService 结构体，注入 `*repository.ExperienceRepository`
5. 实现 CreateExperience, GetExperience, ListExperiences 方法
6. 日志使用 zap.L()，错误返回带上下文的 fmt.Errorf

关键设计：
- Service 层不直接操作 MongoDB，只调用 Repository 层
- 参数校验在 Service 层进行（如标题不能为空）
- 关联操作的幂等性在 Service 层保证
- 暂时不使用 Redis 缓存和 Kafka 事件（简化版），但预留扩展点

**Test scenarios:**

Covers R7. 测试场景：
- 创建需求时标题为空，返回参数校验错误
- 创建成功后返回的 Requirement 对象 ID 已填充
- 查询不存在的需求返回 (nil, error)
- 更新需求时传入不存在的 ID，返回错误
- 列表查询 page=1, page_size=10，返回最多 10 条
- 关联两个需求后，双方的 related_ids 都包含对方
- 创建经验后验证经验数据已持久化

**Verification:** 运行单元测试，验证 Service 层业务逻辑正确，错误处理符合预期。

---

### U3. Gin HTTP Handler 层实现

**Goal:** 实现 HTTP REST API Handler，将 HTTP 请求转换为 gRPC Service 调用，返回 JSON 响应。

**Requirements:** R7

**Dependencies:** U2

**Files:**
- `internal/handler/requirement_handler.go`（新建）
- `internal/handler/experience_handler.go`（新建）

**Approach:**

参考项目中 Handler 层的模式（虽然当前只有 HelloHTTPHandler，但其职责类似）：

1. 创建 RequirementHandler 结构体，注入 `*service.RequirementService`
2. 实现 HTTP 方法：ListRequirements (GET), CreateRequirement (POST), GetRequirement (GET), UpdateRequirement (PUT), LinkRequirements (POST)
3. 使用 Gin 的 `c.BindJSON()` 绑定请求体，`c.JSON()` 返回响应
4. 统一错误响应格式：`{ code: int, msg: string, data: interface{} }`
5. 创建 ExperienceHandler，实现 CreateExperience (POST)

路由设计（在 `internal/router/router.go` 中添加）：
```go
api := r.Group("/api/v1")
{
    api.GET("/requirements", requirementHandler.ListRequirements)
    api.POST("/requirements", requirementHandler.CreateRequirement)
    api.GET("/requirements/:id", requirementHandler.GetRequirement)
    api.PUT("/requirements/:id", requirementHandler.UpdateRequirement)
    api.POST("/requirements/:id/link", requirementHandler.LinkRequirements)
    api.POST("/experiences", experienceHandler.CreateExperience)
}
```

**Test scenarios:**

Covers R7. 测试场景：
- GET /api/v1/requirements 返回 200 和分页列表
- POST /api/v1/requirements 缺少标题，返回 400 错误
- POST /api/v1/requirements 成功，返回 201 和创建的需求
- GET /api/v1/requirements/:id 查询存在的 ID，返回 200 和需求详情
- GET /api/v1/requirements/:id 查询不存在的 ID，返回 404
- PUT /api/v1/requirements/:id 更新成功，返回 200 和更新后的需求
- POST /api/v1/requirements/:id/link 关联成功，返回 200
- POST /api/v1/experiences 创建经验成功，返回 201

**Verification:** 启动服务后用 curl 或 Postman 测试所有 API 端点，验证请求/响应格式正确。

---

### U4. 前端项目初始化

**Goal:** 搭建 React + Vite + TypeScript + Ant Design 前端项目骨架。

**Requirements:** 无（基础设施准备）

**Dependencies:** 无

**Files:**
- `web/package.json`（新建）
- `web/vite.config.ts`（新建）
- `web/tsconfig.json`（新建）
- `web/index.html`（新建）
- `web/src/main.tsx`（新建）
- `web/src/App.tsx`（新建）
- `web/src/pages/RequirementList.tsx`（新建）
- `web/src/pages/RequirementDetail.tsx`（新建）
- `web/src/pages/RequirementForm.tsx`（新建）
- `web/src/services/api.ts`（新建）
- `web/src/types/index.ts`（新建）

**Approach:**

使用 Vite 官方模板初始化项目：
```bash
cd web
npm create vite@latest . -- --template react-ts
npm install antd @ant-design/icons axios react-router-dom
```

项目结构：
- `src/pages/` - 页面组件（列表、详情、表单）
- `src/services/` - API 调用封装
- `src/types/` - TypeScript 类型定义
- `src/components/` - 通用组件（暂不需要，后续再建）

配置 Vite 代理（`vite.config.ts`）：
```typescript
server: {
  proxy: {
    '/api': 'http://localhost:8080'  // 开发环境代理到 Gin 后端
  }
}
```

定义 TypeScript 类型（`src/types/index.ts`）：
```typescript
export interface Requirement {
  id: string;
  title: string;
  description: string;
  related_ids: string[];
  created_at: string;
  updated_at: string;
}
```

**Test scenarios:**

- 运行 `npm run dev`，Vite 开发服务器启动成功
- 访问 http://localhost:5173，显示 Ant Design 默认页面
- 热更新生效，修改代码后浏览器自动刷新

**Verification:** 前端开发服务器正常运行，能访问 Ant Design 组件，API 代理配置生效。

---

### U5. 前端 API 服务层

**Goal:** 封装后端 API 调用，提供类型安全的 JavaScript 接口。

**Requirements:** R7

**Dependencies:** U4

**Files:**
- `web/src/services/api.ts`（新建或更新）

**Approach:**

使用 axios 封装 API 调用：

```typescript
import axios from 'axios';

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
});

export const requirementAPI = {
  list: (page = 1, pageSize = 10) => 
    api.get('/requirements', { params: { page, page_size: pageSize } }),
  
  get: (id: string) => 
    api.get(`/requirements/${id}`),
  
  create: (data: { title: string; description: string }) => 
    api.post('/requirements', data),
  
  update: (id: string, data: Partial<Requirement>) => 
    api.put(`/requirements/${id}`, data),
  
  link: (fromId: string, toId: string) => 
    api.post(`/requirements/${fromId}/link`, { to_requirement_id: toId }),
};

export const experienceAPI = {
  create: (data: CreateExperienceRequest) => 
    api.post('/experiences', data),
};
```

错误处理：
- 统一拦截 4xx/5xx 错误，显示 Ant Design message 提示
- 网络超时显示友好提示

**Test scenarios:**

- 调用 requirementAPI.list() 返回需求列表
- 调用 requirementAPI.create() 创建成功返回新需求
- API 返回 400 错误时，前端显示错误消息
- 网络超时时，前端显示超时提示

**Verification:** 在浏览器控制台测试 API 调用，验证请求发送和响应处理正确。

---

### U6. 需求列表页

**Goal:** 实现需求列表页，分页展示所有需求，点击进入详情页。

**Requirements:** R1

**Dependencies:** U5

**Files:**
- `web/src/pages/RequirementList.tsx`（新建）
- `web/src/App.tsx`（更新，添加路由）

**Approach:**

使用 Ant Design Table 组件展示列表：

```tsx
import { Table, Button, Space } from 'antd';
import { useNavigate } from 'react-router-dom';
import { requirementAPI } from '../services/api';
import type { Requirement } from '../types';

const RequirementList: React.FC = () => {
  const navigate = useNavigate();
  
  const columns = [
    { title: '标题', dataIndex: 'title', key: 'title' },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at' },
    {
      title: '操作',
      key: 'action',
      render: (_, record: Requirement) => (
        <Button onClick={() => navigate(`/requirements/${record.id}`)}>
          查看详情
        </Button>
      ),
    },
  ];

  // 分页状态和加载逻辑...
  
  return <Table columns={columns} dataSource={requirements} pagination={pagination} />;
};
```

路由配置（App.tsx）：
```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import RequirementList from './pages/RequirementList';
import RequirementDetail from './pages/RequirementDetail';
import RequirementForm from './pages/RequirementForm';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<RequirementList />} />
        <Route path="/requirements/new" element={<RequirementForm />} />
        <Route path="/requirements/:id" element={<RequirementDetail />} />
        <Route path="/requirements/:id/edit" element={<RequirementForm mode="edit" />} />
      </Routes>
    </BrowserRouter>
  );
}
```

**Test scenarios:**

Covers R1. 测试场景：
- 页面加载时自动请求 API 获取第一页数据
- 表格显示标题和创建时间列
- 点击"查看详情"按钮跳转到详情页
- 分页器切换页码后重新请求数据
- 无数据时显示空状态

**Verification:** 浏览器访问首页，看到需求列表，点击详情能跳转。

---

### U7. 创建/编辑需求表单

**Goal:** 实现需求创建和编辑的统一表单页面，支持 Markdown 文件上传提取。

**Requirements:** R2, R3

**Dependencies:** U5, U6

**Files:**
- `web/src/pages/RequirementForm.tsx`（新建）

**Approach:**

使用 Ant Design Form 组件：

```tsx
import { Form, Input, Button, Upload, message } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { requirementAPI } from '../services/api';

const RequirementForm: React.FC<{ mode?: 'create' | 'edit' }> = ({ mode = 'create' }) => {
  const navigate = useNavigate();
  const { id } = useParams();
  const [form] = Form.useForm();

  // 如果是编辑模式，加载现有数据
  useEffect(() => {
    if (mode === 'edit' && id) {
      requirementAPI.get(id).then(res => form.setFieldsValue(res.data));
    }
  }, [mode, id]);

  // Markdown 文件上传处理
  const handleMarkdownUpload = async (file: File) => {
    const text = await file.text();
    form.setFieldValue('description', text);
    message.success('Markdown 内容已提取');
    return false; // 阻止自动上传
  };

  const handleSubmit = async (values: any) => {
    try {
      if (mode === 'create') {
        await requirementAPI.create(values);
        message.success('创建成功');
      } else {
        await requirementAPI.update(id!, values);
        message.success('更新成功');
      }
      navigate('/');
    } catch (err) {
      message.error(mode === 'create' ? '创建失败' : '更新失败');
    }
  };

  return (
    <Form form={form} onFinish={handleSubmit}>
      <Form.Item name="title" label="标题" rules={[{ required: true }]}>
        <Input />
      </Form.Item>
      
      <Form.Item label="上传 Markdown">
        <Upload beforeUpload={handleMarkdownUpload} accept=".md">
          <Button icon={<UploadOutlined />}>选择 .md 文件</Button>
        </Upload>
      </Form.Item>
      
      <Form.Item name="description" label="描述">
        <Input.TextArea rows={10} />
      </Form.Item>
      
      <Form.Item>
        <Button type="primary" htmlType="submit">提交</Button>
        <Button onClick={() => navigate('/')}>取消</Button>
      </Form.Item>
    </Form>
  );
};
```

**Test scenarios:**

Covers R2. 测试场景：
- 创建模式：填写标题和描述，提交后创建成功并返回列表
- 标题为空时提交，显示必填校验错误
- 上传 Markdown 文件后，描述字段自动填充文件内容
- 用户可编辑提取后的描述内容
- Covers R3. 编辑模式：加载现有需求数据，修改后提交更新成功

**Verification:** 测试创建和编辑流程，验证表单提交和数据回显正确。

---

### U8. 需求详情页

**Goal:** 实现需求详情页，展示需求信息、关联需求列表、关联经验列表（只读）。

**Requirements:** R4

**Dependencies:** U5, U6, U7

**Files:**
- `web/src/pages/RequirementDetail.tsx`（新建）

**Approach:**

使用 Ant Design Descriptions 和 List 组件：

```tsx
import { Descriptions, List, Card, Button, Divider } from 'antd';
import { useParams, useNavigate } from 'react-router-dom';
import { requirementAPI } from '../services/api';
import type { Requirement } from '../types';

const RequirementDetail: React.FC = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const [requirement, setRequirement] = useState<Requirement | null>(null);
  const [relatedReqs, setRelatedReqs] = useState<Requirement[]>([]);

  useEffect(() => {
    if (id) {
      requirementAPI.get(id).then(res => {
        setRequirement(res.data);
        // 加载关联需求
        if (res.data.related_ids?.length) {
          Promise.all(
            res.data.related_ids.map(rid => requirementAPI.get(rid))
          ).then(results => setRelatedReqs(results.map(r => r.data)));
        }
      });
    }
  }, [id]);

  return (
    <Card
      title="需求详情"
      extra={
        <Button onClick={() => navigate(`/requirements/${id}/edit`)}>编辑</Button>
      }
    >
      <Descriptions column={1}>
        <Descriptions.Item label="标题">{requirement?.title}</Descriptions.Item>
        <Descriptions.Item label="描述">{requirement?.description}</Descriptions.Item>
        <Descriptions.Item label="创建时间">{requirement?.created_at}</Descriptions.Item>
        <Descriptions.Item label="更新时间">{requirement?.updated_at}</Descriptions.Item>
      </Descriptions>

      <Divider>关联需求</Divider>
      <List
        dataSource={relatedReqs}
        renderItem={req => (
          <List.Item>
            <Button type="link" onClick={() => navigate(`/requirements/${req.id}`)}>
              {req.title}
            </Button>
          </List.Item>
        )}
      />

      <Divider>关联经验</Divider>
      <div>暂无关联经验（经验管理通过 API 操作）</div>
    </Card>
  );
};
```

**Test scenarios:**

Covers R4. 测试场景：
- 页面加载时显示需求完整信息
- 有关联需求时，列表展示关联需求的标题，点击可跳转
- 无关联需求时，显示空状态
- 关联经验列表始终显示提示信息（暂无关联经验）
- 点击"编辑"按钮跳转到编辑表单页

**Verification:** 访问详情页，验证所有信息正确展示，关联需求可点击跳转。

---

### U9. 后端静态文件服务和路由整合

**Goal:** 配置 Gin 后端托管前端静态文件，整合所有路由和中间件。

**Requirements:** 无（基础设施整合）

**Dependencies:** U3, U8

**Files:**
- `internal/router/router.go`（更新）
- `cmd/server/main.go`（更新，如果需要）

**Approach:**

1. 在 `internal/router/router.go` 中添加需求和经验的路由：
```go
func SetupRouterWithConfig(...) *gin.Engine {
    // ... 现有中间件 ...
    
    // 初始化 Handler
    requirementHandler := handler.NewRequirementHandler(requirementService)
    experienceHandler := handler.NewExperienceHandler(experienceService)
    
    // API 路由
    api := r.Group("/api/v1")
    {
        // 现有 hello 路由
        api.POST("/hello", httpHandler.SayHello)
        
        // 需求管理路由
        api.GET("/requirements", requirementHandler.ListRequirements)
        api.POST("/requirements", requirementHandler.CreateRequirement)
        api.GET("/requirements/:id", requirementHandler.GetRequirement)
        api.PUT("/requirements/:id", requirementHandler.UpdateRequirement)
        api.POST("/requirements/:id/link", requirementHandler.LinkRequirements)
        api.POST("/experiences", experienceHandler.CreateExperience)
    }
    
    // 前端静态文件服务（生产环境）
    r.StaticFS("/", http.Dir("./web/dist"))
    r.NoRoute(func(c *gin.Context) {
        c.File("./web/dist/index.html") // SPA 路由 fallback
    })
    
    return r
}
```

2. 在 `cmd/server/main.go` 中初始化新的 Service 和 Repository：
```go
// 在 main() 中添加
reqRepo := repository.NewRequirementRepository(mongoClient.GetDatabase())
expRepo := repository.NewExperienceRepository(mongoClient.GetDatabase())
reqService := service.NewRequirementService(reqRepo)
expService := service.NewExperienceService(expRepo)

// 传递给 router
router := router.SetupRouterWithConfig(..., reqService, expService)
```

3. 构建流程：
   - 前端：`cd web && npm run build` 生成 `web/dist/`
   - 后端：`make build` 编译 Go 二进制
   - 部署：单一二进制文件 + `web/dist/` 静态资源

**Test scenarios:**

- 启动后端服务后，访问 http://localhost:8080 显示前端首页
- 访问 http://localhost:8080/api/v1/requirements 返回 API 数据
- 前端路由刷新后不出现 404（SPA fallback 生效）
- 健康检查 /health 仍然可用

**Verification:** 完整构建并启动服务，验证前端页面和 API 都能正常访问。

---

## Scope Boundaries

**Deferred for later:**

- 用户认证和权限控制（简化版实现，所有用户共享需求池）
- 需求优先级、状态、标签等字段的使用（proto 已定义但当前不使用）
- 需求列表的筛选和搜索功能（基础列表即可）
- Word 文档（.doc/.docx）导入（仅支持 Markdown）
- 经验的 UI 操作界面（仅提供 API）
- Redis 缓存层（当前不使用，后续可扩展）
- Kafka 事件发布（当前不使用，后续可扩展）

**Outside this product's identity:**

- 这不是一个通用的任务管理系统，而是专为 Compound Engineering 工作流设计的需求追踪和经验沉淀工具
- 不提供复杂的项目管理功能（如甘特图、里程碑、团队协作者管理等）
- 不涉及 AI 自动生成需求或智能推荐（当前阶段）

## Open Questions

无阻塞性问题。以下事项已明确：
- 技术栈选择：Go + React + MongoDB
- 架构模式：单体应用，Gin 托管静态文件
- 字段范围：最小字段集
- 功能范围：完整实现但简化某些方面（无认证、无筛选等）

## System-Wide Impact

**数据生命周期：**
- 需求数据持久化在 MongoDB，无自动清理策略
- 经验数据同样持久化，与需求通过 ID 关联

**性能考量：**
- 需求列表查询使用 `created_at: -1` 索引，分页避免全表扫描
- 关联需求查询可能需要多次单独查询（N+1 问题），后续可优化为批量查询
- 当前无 Redis 缓存，高并发场景下需评估 MongoDB 压力

**向后兼容性：**
- proto 定义保持不变，未使用的字段保留但不填充
- API 端点设计遵循 RESTful 规范，后续扩展不影响现有接口

## Risks & Dependencies

**Risks:**

1. **前端构建产物部署路径** - Gin 的 `StaticFS` 需要正确指向 `web/dist/` 目录，部署时需确保目录结构正确。缓解：在 Makefile 中添加前端构建步骤，自动化部署流程。

2. **MongoDB 连接配置** - 项目已有 MongoDB 配置，需确认连接字符串和数据库名是否正确。缓解：复用现有的 `configs/config.yaml` 配置。

3. **跨域问题** - 开发环境前端运行在 5173 端口，后端在 8080 端口，需配置 CORS。缓解：已在 `internal/middleware/cors.go` 中实现 CORS 中间件，开发环境允许 localhost。

**Dependencies:**

- 项目已有的 MongoDB、Redis、Kafka 中间件（通过 Docker Compose 启动）
- Node.js 和 npm（前端构建需要，版本 >= 18）
- Go 1.25.5（项目已锁定版本）

## Acceptance Examples

**AE1. 创建需求并查看**

Given: 用户访问需求列表页
When: 用户点击"新建需求"，填写标题"实现用户登录"，上传 Markdown 文件包含详细描述，点击提交
Then: 列表页刷新后显示新需求，点击进入详情页看到标题、描述、创建时间

**AE2. 关联两个需求**

Given: 存在需求 A 和需求 B
When: 用户在需求 A 的详情页点击"编辑"，在关联需求字段输入需求 B 的 ID，保存
Then: 需求 A 的详情页显示关联需求 B，需求 B 的详情页也显示关联需求 A（双向关联）

**AE3. 通过 API 创建经验**

Given: 存在需求 R1
When: 开发者调用 `POST /api/v1/experiences` 传入需求 ID、标题、描述、解决方案
Then: 经验创建成功，后续可通过 API 查询该经验

## Documentation / Operational Notes

**开发者文档：**
- 前端 README（`web/README.md`）需说明如何安装依赖、启动开发服务器、构建生产版本
- API 文档可使用 Swagger/OpenAPI 生成，或在 `docs/api/` 中手动维护

**运维笔记：**
- 部署时需先构建前端（`npm run build`），再编译后端（`make build`）
- MongoDB 需确保 `requirements` 和 `experiences` 集合的索引已创建（Repository 初始化时自动创建）
- 日志输出到 stdout，可通过 systemd 或 Docker 收集

**Makefile 扩展建议：**
```makefile
# 前端相关命令
web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

# 完整构建
build-all: web-build build
```

## Sources / Research

**本地代码模式参考：**
- `internal/model/user.go` - MongoDB 模型定义模式
- `internal/repository/user_repo.go` - Repository 层实现模式
- `internal/service/user_service.go` - Service 层编排模式
- `internal/router/router.go` - 路由和中间件配置

**外部技术文档：**
- Ant Design 5.x 官方文档：https://ant.design/
- Vite + React + TypeScript 模板：https://vitejs.dev/guide/#scaffolding-your-first-vite-project
- MongoDB Go Driver v2 文档：https://www.mongodb.com/docs/drivers/go/current/

**Proto 定义：**
- `api/requirement/v1/requirement.proto` - 需求和经验的 gRPC 接口定义（已有，作为数据模型参考）
