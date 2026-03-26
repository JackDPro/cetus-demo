# Cetus Demo

**English** | [中文](README_zh.md) | [日本語](README_ja.md) | [한국어](README_ko.md)

A step-by-step tutorial for building a complete REST API with [Cetus](https://github.com/JackDPro/cetus). This demo implements user registration, JWT authentication, and user CRUD operations.

## Prerequisites

- Go 1.21+
- PostgreSQL (or MySQL)
- Redis
- OpenSSL

## Getting Started

```bash
git clone https://github.com/JackDPro/cetus-demo.git
cd cetus-demo
go mod tidy
```

## Step-by-Step Guide

This guide walks you through building the project from scratch. If you just want to run the demo, skip to [Quick Run](#quick-run).

---

### Step 1: Initialize the project

```bash
mkdir cetus-demo && cd cetus-demo
go mod init cetus-demo
go get github.com/JackDPro/cetus
go get github.com/gin-contrib/cors
```

Create the project directory structure:

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

### Step 2: Configure environment

Create `.env` file in the project root:

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

> The `OPTIMUS_*` and `JWT_*` values are placeholders — we'll generate real ones in the next two steps.

### Step 3: Generate JWT keys

Cetus JWT requires a **PKCS#8 DER** private key and a **PEM** public key.

Create `storage/jwt_key.sh`:

```bash
#!/bin/sh
# Generate RSA private key (PKCS#1 PEM)
openssl genrsa -out jwt1.pem 2048

# Convert to PKCS#8 DER format (required by cetus)
openssl pkcs8 -topk8 -inform PEM -outform DER \
  -in jwt1.pem -out jwt8-der.key -nocrypt

# Extract public key (PEM)
openssl rsa -in jwt1.pem -pubout -out jwt.pub
```

Run it:

```bash
cd storage && sh jwt_key.sh && cd ..
```

Generated files:

| File | Format | Purpose |
|------|--------|---------|
| `storage/jwt8-der.key` | PKCS#8 DER | Token signing (private key) |
| `storage/jwt.pub` | PEM | Token verification (public key) |

### Step 4: Generate ID obfuscation parameters

Cetus uses [Optimus](https://github.com/pjebs/optimus-go) to encode sequential database IDs into non-guessable integers in API responses (e.g. `1` -> `1580030173`). You need 3 values: `OPTIMUS_PRIME`, `OPTIMUS_INVERSE`, `OPTIMUS_RANDOM`.

Create `storage/optimus_gen.go`:

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

**How to use:**

1. Visit http://primes.utm.edu/lists/small/millions/, download any `.txt` file, open it and **randomly pick a prime number** less than `2,147,483,647`.
2. Pass the chosen prime to the generator:

```bash
go run storage/optimus_gen.go 104393867
```

Output:
```
OPTIMUS_PRIME=104393867
OPTIMUS_INVERSE=1990279033
OPTIMUS_RANDOM=1333095938
```

3. Copy these values into your `.env` file, replacing the placeholders.

> **Important:** Once deployed to production, **never change** these values — all existing encoded IDs in your API will become invalid.

### Step 5: Create the model

Create `model/user.go`:

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

// ToMap implements IModel interface.
// Encodes the database ID before returning to the API response.
func (m *User) ToMap() (map[string]interface{}, error) {
	data, err := m.BaseModel.ToMap(m)
	if err != nil {
		return nil, err
	}
	data["id"] = provider.Hash().Encode(m.Id)
	return data, nil
}

// BeforeSave is a GORM hook that automatically hashes the password before saving.
func (m *User) BeforeSave(_ *gorm.DB) (err error) {
	if m.Password != "" {
		m.Password, err = provider.HashMake(m.Password)
	}
	return
}
```

Key points:
- Embed `model.BaseModel` for serialization helpers
- `ToMap()` implements the `IModel` interface, which is required by `controller.ResponseItem()`
- `provider.Hash().Encode()` encodes the ID so the raw database ID is never exposed
- `BeforeSave()` GORM hook automatically bcrypt-hashes the password

### Step 6: Create request validation structs

These structs use Gin's binding tags for input validation.

Create `request/user_store_request.go`:

```go
package request

type UserStoreRequest struct {
	Username string `binding:"required" form:"username" json:"username"`
	Nickname string `binding:"required" form:"nickname" json:"nickname"`
	Password string `binding:"required,min=8,max=24" form:"password" json:"password"`
}
```

Create `request/user_update_request.go`:

```go
package request

type UserUpdateRequest struct {
	Nickname string `form:"nickname" json:"nickname"`
	Password string `binding:"omitempty,min=8,max=24" form:"password" json:"password"`
	Avatar   string `form:"avatar" json:"avatar"`
}
```

Create `request/auth_password_request.go`:

```go
package request

type AuthPasswordRequest struct {
	Username string `binding:"required" form:"username" json:"username"`
	Password string `binding:"required" form:"password" json:"password"`
}
```

### Step 7: Create providers

Providers encapsulate business logic. The auth provider wraps cetus JWT functionality.

Create `provider/auth_provider.go`:

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

// CreateToken creates a JWT access token + refresh token for the given user.
func (p *AuthProvider) CreateToken(userId uint64) (*jwt.AccessToken, error) {
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return nil, err
	}
	return guard.CreateToken(CetusProvider.Hash().Encode(userId), false)
}

// GetTokenByPassword authenticates a user by username/password and returns tokens.
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

// AttemptAccessToken validates a JWT token and returns the decoded user ID.
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

// DeleteAccessToken revokes a token (logout).
func (p *AuthProvider) DeleteAccessToken(accessToken string) error {
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return err
	}
	return guard.DeleteCredential(accessToken)
}
```

Create `provider/gin_toolkit.go` (optional utility for extracting route parameters):

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

### Step 8: Create middleware

Create `middleware/auth.go` — JWT authentication middleware:

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

How it works:
1. Extract JWT from the `Authorization: Bearer <token>` header
2. Validate the token using `AuthProvider`
3. Store the decoded `user_id` in Gin context for downstream handlers
4. Return 401 if the token is missing or invalid

### Step 9: Create controllers

Create `controller/user_controller.go`:

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

// Store registers a new user.
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

// Me returns the authenticated user's profile.
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

// Show returns a user by encoded ID.
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

// Update modifies the authenticated user's profile.
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

Create `controller/auth_controller.go`:

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

// AuthByPassword authenticates a user with username/password and returns JWT tokens.
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

// Logout revokes the current access token.
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

### Step 10: Create database migration

Create `db/migrate.go`:

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

Create the database and run the migration:

```bash
# PostgreSQL
createdb cetus

# MySQL
# mysql -u root -p -e "CREATE DATABASE cetus"

go run db/migrate.go
```

### Step 11: Create the entry point

Create `main.go`:

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

	// CORS
	corsConf := cors.DefaultConfig()
	corsConf.AllowAllOrigins = true
	corsConf.AllowHeaders = []string{"Authorization", "Content-Type", "X-Requested-With"}
	corsConf.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	router.Use(cors.New(corsConf))

	// Request ID middleware (from cetus)
	router.Use(middleware.RequestId())

	// --- Public routes (no auth) ---
	router.GET("/probe", probe.Show)
	router.POST("/users", userCtr.Store)
	router.POST("/auth/password", authCtr.AuthByPassword)

	// --- Protected routes (require JWT) ---
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

### Step 12: Run!

```bash
go run main.go
```

The server starts at `http://localhost:9001`.

---

## Quick Run

If you just want to run the demo without building from scratch:

```bash
git clone https://github.com/JackDPro/cetus-demo.git
cd cetus-demo
go mod tidy
cp .env.example .env        # Edit .env with your DB/Redis credentials
cd storage && sh jwt_key.sh && cd ..   # Generate JWT keys
go run storage/optimus_gen.go 104393867  # Generate ID hashing params, copy to .env
createdb cetus               # Create database
go run db/migrate.go         # Run migration
go run main.go               # Start server
```

## API Reference

### Health Check

```bash
curl http://localhost:9001/probe
```

### Register

```bash
curl -X POST http://localhost:9001/users \
  -H "Content-Type: application/json" \
  -d '{"username": "jack", "nickname": "Jack", "password": "12345678"}'
```

Response `201 Created`:
```json
{"id": 1580030173}
```

### Login

```bash
curl -X POST http://localhost:9001/auth/password \
  -H "Content-Type: application/json" \
  -d '{"username": "jack", "password": "12345678"}'
```

Response:
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

### Get Current User

```bash
curl http://localhost:9001/users/me \
  -H "Authorization: Bearer <access_token>"
```

### Get User by ID

```bash
curl http://localhost:9001/users/1580030173 \
  -H "Authorization: Bearer <access_token>"
```

### Update User

```bash
curl -X PUT http://localhost:9001/users/1580030173 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"nickname": "Jack Pro"}'
```

### Logout

```bash
curl -X POST http://localhost:9001/auth/logout \
  -H "Authorization: Bearer <access_token>"
```

## Project Structure

```
cetus-demo/
├── main.go                  # Entry point & router setup
├── controller/
│   ├── user_controller.go   # User CRUD handlers
│   └── auth_controller.go   # Login / logout handlers
├── model/
│   └── user.go              # User model (BaseModel + GORM hooks)
├── middleware/
│   ├── auth.go              # JWT authentication middleware
│   └── localization.go      # Accept-Language middleware
├── provider/
│   ├── auth_provider.go     # JWT token operations
│   └── gin_toolkit.go       # Request parameter helpers
├── request/
│   ├── user_store_request.go
│   ├── user_update_request.go
│   └── auth_password_request.go
├── db/
│   └── migrate.go           # Database auto-migration
└── storage/
    ├── jwt_key.sh           # JWT key generation script
    ├── optimus_gen.go       # ID hashing parameter generator
    ├── jwt8-der.key         # RSA private key (generated)
    └── jwt.pub              # RSA public key (generated)
```

## Cetus Features Used

| Feature | Where | What it does |
|---------|-------|--------------|
| Config | `main.go` | `config.GetAppConfig()`, `config.GetApiConfig()` |
| Database | `controller/user_controller.go` | `provider.GetOrm().Db` for GORM queries |
| JWT | `provider/auth_provider.go` | `jwt.GetJwtGuard()` for token create/validate/revoke |
| Password hash | `model/user.go` | `provider.HashMake()` in `BeforeSave()` hook |
| ID obfuscation | `model/user.go` | `provider.Hash().Encode/Decode()` |
| Request ID | `main.go` | `middleware.RequestId()` adds trace ID to every request |
| Response helpers | All controllers | `ResponseItem()`, `ResponseCreated()`, `ResponseSuccess()`, etc. |
| Logging | `main.go` | `provider.GetLogger()` structured logging |
| BaseModel | `model/user.go` | `ToMap()` serialization with json tags |

## License

MIT