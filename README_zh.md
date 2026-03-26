# Cetus Demo

[English](README.md) | **中文** | [日本語](README_ja.md) | [한국어](README_ko.md)

使用 [Cetus](https://github.com/JackDPro/cetus) 从零构建完整 REST API 的分步教程。本示例实现了用户注册、JWT 认证和用户 CRUD 操作。

## 环境要求

- Go 1.21+
- PostgreSQL（或 MySQL）
- Redis
- OpenSSL

## 获取项目

```bash
git clone https://github.com/JackDPro/cetus-demo.git
cd cetus-demo
go mod tidy
```

## 分步指南

本指南带你从零开始构建整个项目。如果只想快速运行，请跳到[快速运行](#快速运行)。

---

### 第 1 步：初始化项目

```bash
mkdir cetus-demo && cd cetus-demo
go mod init cetus-demo
go get github.com/JackDPro/cetus
go get github.com/gin-contrib/cors
```

创建项目目录结构：

```
cetus-demo/
├── controller/
├── model/
├── middleware/
├── provider/
├── request/
├── db/
└── storage/
```

### 第 2 步：配置环境变量

在项目根目录创建 `.env` 文件：

```env
APP_NAME=cetus-demo
APP_ENV=dev
APP_DATA_ROOT=storage

LOG_CONSOLE_OUT=true
LOG_FILE_OUT=false
LOG_LEVEL=debug
LOG_FORMAT=json

DB_TYPE=postgres
DB_HOST=127.0.0.1
DB_PORT=5432
DB_DATABASE=cetus
DB_USERNAME=postgres
DB_PASSWORD=password

REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_DATABASE=0
REDIS_PASSWORD=password

JWT_CERT_PATH=storage/jwt.pub
JWT_KEY_PATH=storage/jwt8-der.key
JWT_EXPIRES_IN=72
JWT_REDIS_PREFIX=auth
JWT_ISSUE=https://cetus.com

OPTIMUS_PRIME=11
OPTIMUS_INVERSE=22
OPTIMUS_RANDOM=33

SERVER_HTTP_PORT=9001
```

> `OPTIMUS_*` 和 `JWT_*` 的值是占位符，接下来两步会生成真实的值。

### 第 3 步：生成 JWT 密钥

Cetus JWT 需要 **PKCS#8 DER** 格式的私钥和 **PEM** 格式的公钥。

创建 `storage/jwt_key.sh`：

```bash
#!/bin/sh
# 生成 RSA 私钥（PKCS#1 PEM 格式）
openssl genrsa -out jwt1.pem 2048

# 转换为 PKCS#8 DER 格式（cetus 要求的格式）
openssl pkcs8 -topk8 -inform PEM -outform DER \
  -in jwt1.pem -out jwt8-der.key -nocrypt

# 导出公钥（PEM 格式）
openssl rsa -in jwt1.pem -pubout -out jwt.pub
```

执行：

```bash
cd storage && sh jwt_key.sh && cd ..
```

生成的文件：

| 文件 | 格式 | 用途 |
|------|------|------|
| `storage/jwt8-der.key` | PKCS#8 DER | 令牌签名（私钥） |
| `storage/jwt.pub` | PEM | 令牌验证（公钥） |

### 第 4 步：生成 ID 混淆参数

Cetus 使用 [Optimus](https://github.com/pjebs/optimus-go) 将数据库自增 ID 编码为不可猜测的整数（例如 `1` -> `1580030173`）。需要 3 个值：`OPTIMUS_PRIME`、`OPTIMUS_INVERSE`、`OPTIMUS_RANDOM`。

创建 `storage/optimus_gen.go`：

```go
package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strconv"
)

const maxInt = uint64(1<<31 - 1) // 2,147,483,647

func modInverse(prime uint64) uint64 {
	p := big.NewInt(int64(prime))
	max := big.NewInt(int64(maxInt + 1))
	var i big.Int
	return i.ModInverse(p, max).Uint64()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run storage/optimus_gen.go <prime>")
		fmt.Fprintln(os.Stderr, "Prime numbers: http://primes.utm.edu/lists/small/millions/")
		os.Exit(1)
	}
	prime, err := strconv.ParseUint(os.Args[1], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid prime: %s\n", os.Args[1])
		os.Exit(1)
	}
	upper := big.NewInt(int64(maxInt - 1))
	r, _ := rand.Int(rand.Reader, upper)
	random := r.Uint64() + 1

	fmt.Printf("OPTIMUS_PRIME=%d\n", prime)
	fmt.Printf("OPTIMUS_INVERSE=%d\n", modInverse(prime))
	fmt.Printf("OPTIMUS_RANDOM=%d\n", random)
}
```

**使用方法：**

1. 访问 http://primes.utm.edu/lists/small/millions/，下载任意一个 `.txt` 文件，打开后**随机选取一个**小于 `2,147,483,647` 的质数。
2. 将选好的质数传给生成工具：

```bash
go run storage/optimus_gen.go 104393867
```

输出：
```
OPTIMUS_PRIME=104393867
OPTIMUS_INVERSE=1990279033
OPTIMUS_RANDOM=1333095938
```

3. 将这些值复制到 `.env` 文件中，替换占位符。

> **重要提示：** 一旦部署到生产环境，**切勿更改**这些值——否则所有已编码的 ID 将失效。

### 第 5 步：创建模型

创建 `model/user.go`：

```go
package model

import (
	"time"

	"github.com/JackDPro/cetus/model"
	"github.com/JackDPro/cetus/provider"
	"gorm.io/gorm"
)

type User struct {
	model.BaseModel
	Id        uint64         `json:"id" gorm:"primaryKey"`
	Nickname  string         `json:"nickname"`
	Username  string         `json:"username" gorm:"unique"`
	Password  string         `binding:"required"`
	Avatar    string         `json:"avatar"`
	Status    int            `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt
}

// ToMap 实现 IModel 接口
// 返回 API 响应前对数据库 ID 进行编码
func (m *User) ToMap() (map[string]interface{}, error) {
	data, err := m.BaseModel.ToMap(m)
	if err != nil {
		return nil, err
	}
	data["id"] = provider.Hash().Encode(m.Id)
	return data, nil
}

// BeforeSave 是 GORM 钩子，保存前自动对密码进行哈希
func (m *User) BeforeSave(_ *gorm.DB) (err error) {
	if m.Password != "" {
		m.Password, err = provider.HashMake(m.Password)
	}
	return
}
```

要点：
- 嵌入 `model.BaseModel` 获得序列化辅助方法
- `ToMap()` 实现 `IModel` 接口，`controller.ResponseItem()` 需要该接口
- `provider.Hash().Encode()` 对 ID 编码，避免暴露真实数据库 ID
- `BeforeSave()` GORM 钩子自动对密码进行 bcrypt 哈希

### 第 6 步：创建请求验证结构体

使用 Gin 的 binding 标签进行输入验证。

创建 `request/user_store_request.go`：

```go
package request

type UserStoreRequest struct {
	Username string `binding:"required" form:"username" json:"username"`
	Nickname string `binding:"required" form:"nickname" json:"nickname"`
	Password string `binding:"required,min=8,max=24" form:"password" json:"password"`
}
```

创建 `request/user_update_request.go`：

```go
package request

type UserUpdateRequest struct {
	Nickname string `form:"nickname" json:"nickname"`
	Password string `binding:"omitempty,min=8,max=24" form:"password" json:"password"`
	Avatar   string `form:"avatar" json:"avatar"`
}
```

创建 `request/auth_password_request.go`：

```go
package request

type AuthPasswordRequest struct {
	Username string `binding:"required" form:"username" json:"username"`
	Password string `binding:"required" form:"password" json:"password"`
}
```

### 第 7 步：创建 Provider

Provider 封装业务逻辑。Auth Provider 封装了 cetus 的 JWT 功能。

创建 `provider/auth_provider.go`：

```go
package provider

import (
	"cetus-demo/model"
	"fmt"

	"github.com/JackDPro/cetus/jwt"
	CetusProvider "github.com/JackDPro/cetus/provider"
)

type AuthProvider struct{}

func NewAuthProvider() *AuthProvider {
	return &AuthProvider{}
}

// CreateToken 为指定用户创建 JWT 访问令牌 + 刷新令牌
func (p *AuthProvider) CreateToken(userId uint64) (*jwt.AccessToken, error) {
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return nil, err
	}
	return guard.CreateToken(CetusProvider.Hash().Encode(userId), false)
}

// GetTokenByPassword 通过用户名/密码认证并返回令牌
func (p *AuthProvider) GetTokenByPassword(username, password string) (*jwt.AccessToken, error) {
	var item = &model.User{}
	CetusProvider.GetOrm().Db.Where("username=?", username).First(&item)
	if item.Id == 0 {
		return nil, fmt.Errorf("not found user")
	}
	if err := CetusProvider.HashCheck(password, item.Password); err != nil {
		return nil, fmt.Errorf("invalid password")
	}
	return p.CreateToken(item.Id)
}

// AttemptAccessToken 验证 JWT 令牌并返回解码后的用户 ID
func (p *AuthProvider) AttemptAccessToken(accessToken string) (uint64, error) {
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return 0, err
	}
	token, err := guard.Attempt(accessToken)
	if err != nil {
		return 0, err
	}
	return CetusProvider.Hash().Decode(token.UserId), nil
}

// DeleteAccessToken 撤销令牌（退出登录）
func (p *AuthProvider) DeleteAccessToken(accessToken string) error {
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return err
	}
	return guard.DeleteCredential(accessToken)
}
```

创建 `provider/gin_toolkit.go`（可选的路由参数提取工具）：

```go
package provider

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetIdFromGin[T any](c *gin.Context, convertFunc func(string) (T, error)) (T, error) {
	idStr := c.Param("id")
	result, err := convertFunc(idStr)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("failed to convert id: %w", err)
	}
	return result, nil
}

func ConvertToUInt64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}
```

### 第 8 步：创建中间件

创建 `middleware/auth.go` —— JWT 认证中间件：

```go
package middleware

import (
	"cetus-demo/provider"
	"strings"

	"github.com/JackDPro/cetus/controller"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		tokenArr := strings.Split(tokenStr, "Bearer ")
		if len(tokenArr) != 2 {
			controller.ResponseUnauthorized(c)
			return
		}
		p := provider.NewAuthProvider()
		userId, err := p.AttemptAccessToken(tokenArr[1])
		if err != nil {
			controller.ResponseUnauthorized(c)
			return
		}
		c.Set("user_id", userId)
		c.Next()
	}
}
```

工作流程：
1. 从 `Authorization: Bearer <token>` 请求头中提取 JWT
2. 使用 `AuthProvider` 验证令牌
3. 将解码后的 `user_id` 存入 Gin 上下文，供下游处理器使用
4. 令牌缺失或无效时返回 401

### 第 9 步：创建控制器

创建 `controller/user_controller.go`：

```go
package controller

import (
	"cetus-demo/model"
	AppProvider "cetus-demo/provider"
	"cetus-demo/request"

	"github.com/JackDPro/cetus/controller"
	"github.com/JackDPro/cetus/provider"
	"github.com/gin-gonic/gin"
)

type UserController struct{}

func NewUserController() *UserController {
	return &UserController{}
}

// Store 注册新用户
func (ctr *UserController) Store(c *gin.Context) {
	var payload = &request.UserStoreRequest{}
	if err := c.ShouldBind(payload); err != nil {
		controller.ResponseUnprocessable(c, 1, "params is invalid", err)
		return
	}
	user := model.User{
		Username: payload.Username,
		Nickname: payload.Nickname,
		Password: payload.Password,
		Status:   1,
	}
	if err := provider.GetOrm().Db.Create(&user).Error; err != nil {
		controller.ResponseInternalError(c, 1000, "create user failed", err)
		return
	}
	controller.ResponseCreated(c, provider.Hash().Encode(user.Id))
}

// Me 返回当前认证用户的信息
func (ctr *UserController) Me(c *gin.Context) {
	var user model.User
	userId, ok := c.Get("user_id")
	if !ok {
		controller.ResponseUnauthorized(c)
		return
	}
	provider.GetOrm().Db.Where("id", userId).First(&user)
	if user.Id == 0 {
		controller.ResponseNotFound(c, "user not found")
		return
	}
	controller.ResponseItem(c, &user)
}

// Show 根据编码后的 ID 返回用户信息
func (ctr *UserController) Show(c *gin.Context) {
	userId, err := AppProvider.GetIdFromGin[uint64](c, AppProvider.ConvertToUInt64)
	if err != nil {
		controller.ResponseUnprocessable(c, 1, "invalid id", err)
		return
	}
	var user model.User
	decodeId := provider.Hash().Decode(userId)
	provider.GetOrm().Db.Where("id", decodeId).First(&user)
	if user.Id == 0 {
		controller.ResponseNotFound(c, "user not found")
		return
	}
	controller.ResponseItem(c, &user)
}

// Update 修改用户信息
func (ctr *UserController) Update(c *gin.Context) {
	var user model.User
	userId, ok := c.Get("user_id")
	if !ok {
		controller.ResponseUnauthorized(c)
		return
	}
	provider.GetOrm().Db.Where("id", userId).First(&user)
	if user.Id == 0 {
		controller.ResponseNotFound(c, "user not found")
		return
	}
	var payload = &request.UserUpdateRequest{}
	if err := c.ShouldBind(payload); err != nil {
		controller.ResponseUnprocessable(c, 1, "params is invalid", err)
		return
	}
	if payload.Nickname != "" {
		user.Nickname = payload.Nickname
	}
	if payload.Password != "" {
		user.Password = payload.Password
	}
	if payload.Avatar != "" {
		user.Avatar = payload.Avatar
	}
	if err := provider.GetOrm().Db.Save(&user).Error; err != nil {
		controller.ResponseInternalError(c, 1001, "update user failed", err)
		return
	}
	controller.ResponseSuccess(c)
}
```

创建 `controller/auth_controller.go`：

```go
package controller

import (
	AppProvider "cetus-demo/provider"
	"cetus-demo/request"
	"strings"

	"github.com/JackDPro/cetus/controller"
	"github.com/gin-gonic/gin"
)

type AuthController struct{}

func NewAuthController() *AuthController {
	return &AuthController{}
}

// AuthByPassword 通过用户名/密码认证并返回 JWT 令牌
func (ctr *AuthController) AuthByPassword(c *gin.Context) {
	var payload = &request.AuthPasswordRequest{}
	if err := c.ShouldBind(payload); err != nil {
		controller.ResponseUnprocessable(c, 1, "params is invalid", err)
		return
	}
	authProvider := AppProvider.NewAuthProvider()
	accessToken, err := authProvider.GetTokenByPassword(payload.Username, payload.Password)
	if err != nil {
		controller.ResponseUnprocessable(c, 1000, "auth failed", err)
		return
	}
	controller.ResponseItem(c, accessToken)
}

// Logout 撤销当前访问令牌
func (ctr *AuthController) Logout(c *gin.Context) {
	tokenStr := c.GetHeader("Authorization")
	tokenArr := strings.Split(tokenStr, "Bearer ")
	if len(tokenArr) != 2 {
		controller.ResponseUnauthorized(c)
		return
	}
	p := AppProvider.NewAuthProvider()
	if err := p.DeleteAccessToken(tokenArr[1]); err != nil {
		controller.ResponseInternalError(c, 1000, "logout failed", err)
		return
	}
	controller.ResponseSuccess(c)
}
```

### 第 10 步：创建数据库迁移

创建 `db/migrate.go`：

```go
package main

import (
	"cetus-demo/model"
	"log"

	"github.com/JackDPro/cetus/provider"
)

func main() {
	mysql := provider.GetOrm()
	tables := []interface{}{
		&model.User{},
	}
	for _, table := range tables {
		err := mysql.Db.AutoMigrate(table)
		if err != nil {
			log.Fatalf("create %T table failed: %v\n", table, err)
		}
	}
}
```

创建数据库并运行迁移：

```bash
# PostgreSQL
createdb cetus

# MySQL
# mysql -u root -p -e "CREATE DATABASE cetus"

go run db/migrate.go
```

### 第 11 步：创建入口文件

创建 `main.go`：

```go
package main

import (
	"cetus-demo/controller"
	AppMiddleware "cetus-demo/middleware"
	"errors"
	"fmt"

	"github.com/JackDPro/cetus/config"
	CetusController "github.com/JackDPro/cetus/controller"
	"github.com/JackDPro/cetus/middleware"
	"github.com/JackDPro/cetus/provider"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var probe *CetusController.ProbeController
var userCtr *controller.UserController
var authCtr *controller.AuthController

func init() {
	provider.GetLogger().Infow("init success")
	probe = CetusController.NewProbeController()
	authCtr = controller.NewAuthController()
	userCtr = controller.NewUserController()
}

func StartServer() error {
	appConf := config.GetAppConfig()
	apiConf := config.GetApiConfig()

	if appConf.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// 跨域配置
	corsConf := cors.DefaultConfig()
	corsConf.AllowAllOrigins = true
	corsConf.AllowHeaders = []string{"Authorization", "Content-Type", "X-Requested-With"}
	corsConf.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	router.Use(cors.New(corsConf))

	// 请求 ID 中间件（来自 cetus）
	router.Use(middleware.RequestId())

	// --- 公开路由（无需认证） ---
	router.GET("/probe", probe.Show)
	router.POST("/users", userCtr.Store)
	router.POST("/auth/password", authCtr.AuthByPassword)

	// --- 受保护路由（需要 JWT） ---
	authorized := router.Use(AppMiddleware.AuthMiddleware())
	authorized.POST("/auth/logout", authCtr.Logout)
	authorized.GET("/users/me", userCtr.Me)
	authorized.GET("/users/:id", userCtr.Show)
	authorized.PUT("/users/:id", userCtr.Update)

	address := fmt.Sprintf("0.0.0.0:%d", apiConf.HttpPort)
	provider.GetLogger().Infow("start api server success", "address", address)

	if err := router.Run(address); err != nil {
		return errors.New("start http server failed error=" + err.Error())
	}
	return nil
}

func main() {
	if err := StartServer(); err != nil {
		panic(err)
	}
}
```

### 第 12 步：运行！

```bash
go run main.go
```

服务启动在 `http://localhost:9001`。

---

## 快速运行

如果只想快速运行，不需要从零构建：

```bash
git clone https://github.com/JackDPro/cetus-demo.git
cd cetus-demo
go mod tidy
cp .env.example .env                     # 编辑 .env，填写数据库/Redis 信息
cd storage && sh jwt_key.sh && cd ..     # 生成 JWT 密钥
go run storage/optimus_gen.go 104393867  # 生成 ID 混淆参数，复制到 .env
createdb cetus                           # 创建数据库
go run db/migrate.go                     # 运行迁移
go run main.go                           # 启动服务
```

## API 接口

### 健康检查

```bash
curl http://localhost:9001/probe
```

### 注册

```bash
curl -X POST http://localhost:9001/users \
  -H "Content-Type: application/json" \
  -d '{"username": "jack", "nickname": "Jack", "password": "12345678"}'
```

响应 `201 Created`：
```json
{"id": 1580030173}
```

### 登录

```bash
curl -X POST http://localhost:9001/auth/password \
  -H "Content-Type: application/json" \
  -d '{"username": "jack", "password": "12345678"}'
```

响应：
```json
{
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJSUzI1NiIs...",
    "token_type": "bearer",
    "expires_in": 259200
  }
}
```

### 获取当前用户

```bash
curl http://localhost:9001/users/me \
  -H "Authorization: Bearer <access_token>"
```

### 根据 ID 获取用户

```bash
curl http://localhost:9001/users/1580030173 \
  -H "Authorization: Bearer <access_token>"
```

### 更新用户

```bash
curl -X PUT http://localhost:9001/users/1580030173 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"nickname": "Jack Pro"}'
```

### 退出登录

```bash
curl -X POST http://localhost:9001/auth/logout \
  -H "Authorization: Bearer <access_token>"
```

## 项目结构

```
cetus-demo/
├── main.go                  # 入口文件 & 路由配置
├── controller/
│   ├── user_controller.go   # 用户 CRUD 处理器
│   └── auth_controller.go   # 登录 / 退出处理器
├── model/
│   └── user.go              # 用户模型（BaseModel + GORM 钩子）
├── middleware/
│   ├── auth.go              # JWT 认证中间件
│   └── localization.go      # Accept-Language 中间件
├── provider/
│   ├── auth_provider.go     # JWT 令牌操作
│   └── gin_toolkit.go       # 请求参数辅助函数
├── request/
│   ├── user_store_request.go
│   ├── user_update_request.go
│   └── auth_password_request.go
├── db/
│   └── migrate.go           # 数据库自动迁移
└── storage/
    ├── jwt_key.sh           # JWT 密钥生成脚本
    ├── optimus_gen.go       # ID 混淆参数生成器
    ├── jwt8-der.key         # RSA 私钥（生成的）
    └── jwt.pub              # RSA 公钥（生成的）
```

## 使用的 Cetus 功能

| 功能 | 位置 | 说明 |
|------|------|------|
| 配置管理 | `main.go` | `config.GetAppConfig()`、`config.GetApiConfig()` |
| 数据库 | `controller/user_controller.go` | `provider.GetOrm().Db` 进行 GORM 查询 |
| JWT | `provider/auth_provider.go` | `jwt.GetJwtGuard()` 创建/验证/撤销令牌 |
| 密码哈希 | `model/user.go` | `BeforeSave()` 钩子中调用 `provider.HashMake()` |
| ID 混淆 | `model/user.go` | `provider.Hash().Encode/Decode()` |
| 请求 ID | `main.go` | `middleware.RequestId()` 为每个请求添加追踪 ID |
| 响应辅助 | 所有控制器 | `ResponseItem()`、`ResponseCreated()`、`ResponseSuccess()` 等 |
| 日志 | `main.go` | `provider.GetLogger()` 结构化日志 |
| BaseModel | `model/user.go` | `ToMap()` 基于 json 标签的序列化 |

## 开源许可

MIT