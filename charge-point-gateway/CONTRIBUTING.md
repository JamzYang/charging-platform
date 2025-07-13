# 贡献指南

感谢您对充电桩网关项目的贡献！本文档将指导您如何参与项目开发。

## 开发环境设置

### 前置要求

- Go 1.21+
- Git
- Docker (可选，用于本地测试)
- Make

### 环境配置

1. 克隆项目
```bash
git clone <repository-url>
cd charge-point-gateway
```

2. 安装开发工具
```bash
make install-tools
```

3. 初始化项目
```bash
make init
```

## 代码规范

### Go 代码规范

我们遵循标准的Go代码规范：

- 使用 `gofmt` 格式化代码
- 使用 `goimports` 管理导入
- 遵循 [Effective Go](https://golang.org/doc/effective_go.html) 指南
- 使用 `golangci-lint` 进行静态分析

### 命名规范

- **包名**: 小写，简短，有意义
- **函数名**: 驼峰命名，公共函数首字母大写
- **变量名**: 驼峰命名，简洁明了
- **常量名**: 全大写，下划线分隔
- **接口名**: 以 `er` 结尾（如 `Handler`, `Manager`）

### 注释规范

- 所有公共函数、类型、常量必须有文档注释
- 注释应该解释"为什么"而不仅仅是"是什么"
- 使用完整的句子，以被注释的名称开头

```go
// ConnectionManager 管理WebSocket连接的生命周期
// 它负责连接的注册、注销和状态维护
type ConnectionManager interface {
    // RegisterConnection 注册一个新的WebSocket连接
    // 如果连接已存在，返回错误
    RegisterConnection(conn *Connection) error
}
```

## 测试规范

### 测试覆盖率

- 单元测试覆盖率必须 >= 80%
- 所有公共函数都必须有测试
- 关键业务逻辑必须有完整的测试用例

### 测试文件组织

- 测试文件与源文件在同一包中
- 测试文件以 `_test.go` 结尾
- 测试函数以 `Test` 开头

### 测试用例编写

```go
func TestConnectionManager_RegisterConnection(t *testing.T) {
    tests := []struct {
        name    string
        setup   func() *ConnectionManager
        conn    *Connection
        wantErr bool
    }{
        {
            name: "successful registration",
            setup: func() *ConnectionManager {
                return NewConnectionManager()
            },
            conn: &Connection{ID: "test-1"},
            wantErr: false,
        },
        // 更多测试用例...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cm := tt.setup()
            err := cm.RegisterConnection(tt.conn)
            if (err != nil) != tt.wantErr {
                t.Errorf("RegisterConnection() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## 提交规范

### 提交信息格式

使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### 提交类型

- `feat`: 新功能
- `fix`: 错误修复
- `docs`: 文档更新
- `style`: 代码格式化
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建过程或辅助工具的变动

### 示例

```
feat(gateway): add WebSocket connection management

- Implement connection registration and deregistration
- Add connection health check mechanism
- Support graceful connection shutdown

Closes #123
```

## 开发流程

### 分支策略

- `main`: 主分支，保持稳定
- `develop`: 开发分支，集成新功能
- `feature/*`: 功能分支
- `hotfix/*`: 热修复分支

### 开发步骤

1. 从 `develop` 分支创建功能分支
```bash
git checkout develop
git pull origin develop
git checkout -b feature/your-feature-name
```

2. 开发功能并编写测试
```bash
# 开发代码
# 编写测试
make test
make lint
```

3. 提交代码
```bash
git add .
git commit -m "feat: your feature description"
```

4. 推送分支并创建 Pull Request
```bash
git push origin feature/your-feature-name
```

### Pull Request 要求

- 描述清楚变更内容和原因
- 确保所有测试通过
- 确保代码检查通过
- 至少一个代码审查者批准

## 代码审查

### 审查要点

- 代码逻辑正确性
- 性能考虑
- 安全性检查
- 测试覆盖率
- 文档完整性
- 代码风格一致性

### 审查流程

1. 自动化检查（CI/CD）
2. 同行代码审查
3. 技术负责人审查（如需要）
4. 合并到目标分支

## 问题报告

### Bug 报告

使用 GitHub Issues 报告 Bug，包含：

- 问题描述
- 复现步骤
- 期望行为
- 实际行为
- 环境信息
- 相关日志

### 功能请求

- 功能描述
- 使用场景
- 预期收益
- 实现建议

## 发布流程

1. 更新版本号
2. 更新 CHANGELOG
3. 创建 Release Tag
4. 构建和发布 Docker 镜像
5. 部署到测试环境验证
6. 发布到生产环境

## 联系方式

如有问题，请通过以下方式联系：

- GitHub Issues
- 项目邮件列表
- 技术讨论群

感谢您的贡献！
