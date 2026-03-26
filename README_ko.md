# Cetus Demo

[English](README.md) | [中文](README_zh.md) | [日本語](README_ja.md) | **한국어**

[Cetus](https://github.com/JackDPro/cetus)를 사용하여 완전한 REST API를 처음부터 구축하는 단계별 튜토리얼. 사용자 등록, JWT 인증, 사용자 CRUD 작업을 구현합니다.

## 사전 요구 사항

- Go 1.21+
- PostgreSQL (또는 MySQL)
- Redis
- OpenSSL

## 프로젝트 가져오기

```bash
git clone https://github.com/JackDPro/cetus-demo.git
cd cetus-demo
go mod tidy
```

## 단계별 가이드

이 가이드는 프로젝트를 처음부터 구축하는 과정을 안내합니다. 바로 실행하려면 [빠른 실행](#빠른-실행)으로 이동하세요.

---

### 1단계: 프로젝트 초기화

```bash
mkdir cetus-demo && cd cetus-demo
go mod init cetus-demo
go get github.com/JackDPro/cetus
go get github.com/gin-contrib/cors
```

프로젝트 디렉토리 구조 생성:

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

### 2단계: 환경 설정

프로젝트 루트에 `.env` 파일 생성:

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

> `OPTIMUS_*`와 `JWT_*` 값은 임시 값입니다. 다음 두 단계에서 실제 값을 생성합니다.

### 3단계: JWT 키 생성

Cetus JWT는 **PKCS#8 DER** 형식의 개인 키와 **PEM** 형식의 공개 키가 필요합니다.

`storage/jwt_key.sh` 생성:

```bash
#!/bin/sh
# RSA 개인 키 생성 (PKCS#1 PEM 형식)
openssl genrsa -out jwt1.pem 2048

# PKCS#8 DER 형식으로 변환 (cetus 필수 형식)
openssl pkcs8 -topk8 -inform PEM -outform DER \
  -in jwt1.pem -out jwt8-der.key -nocrypt

# 공개 키 추출 (PEM 형식)
openssl rsa -in jwt1.pem -pubout -out jwt.pub
```

실행:

```bash
cd storage && sh jwt_key.sh && cd ..
```

생성되는 파일:

| 파일 | 형식 | 용도 |
|------|------|------|
| `storage/jwt8-der.key` | PKCS#8 DER | 토큰 서명 (개인 키) |
| `storage/jwt.pub` | PEM | 토큰 검증 (공개 키) |

### 4단계: ID 난독화 파라미터 생성

Cetus는 [Optimus](https://github.com/pjebs/optimus-go)를 사용하여 데이터베이스 자동 증가 ID를 추측 불가능한 정수로 인코딩합니다 (예: `1` -> `1580030173`). 3개의 값이 필요합니다: `OPTIMUS_PRIME`, `OPTIMUS_INVERSE`, `OPTIMUS_RANDOM`.

`storage/optimus_gen.go` 생성:

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

**사용 방법:**

1. http://primes.utm.edu/lists/small/millions/ 에 접속하여 아무 `.txt` 파일을 다운로드하고, `2,147,483,647` 미만의 소수를 **하나 임의로 선택**합니다.
2. 선택한 소수를 생성기에 전달합니다:

```bash
go run storage/optimus_gen.go 104393867
```

출력:
```
OPTIMUS_PRIME=104393867
OPTIMUS_INVERSE=1990279033
OPTIMUS_RANDOM=1333095938
```

3. 이 값들을 `.env` 파일에 복사하여 임시 값을 교체합니다.

> **중요:** 프로덕션 배포 후 이 값을 **절대 변경하지 마세요**. 기존의 모든 인코딩된 ID가 무효화됩니다.

### 5단계: 모델 생성

`model/user.go` 생성:

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

// ToMap은 IModel 인터페이스를 구현
// API 응답 반환 전에 데이터베이스 ID를 인코딩
func (m *User) ToMap() (map[string]interface{}, error) {
	data, err := m.BaseModel.ToMap(m)
	if err != nil {
		return nil, err
	}
	data["id"] = provider.Hash().Encode(m.Id)
	return data, nil
}

// BeforeSave는 GORM 훅으로, 저장 전에 비밀번호를 자동 해싱
func (m *User) BeforeSave(_ *gorm.DB) (err error) {
	if m.Password != "" {
		m.Password, err = provider.HashMake(m.Password)
	}
	return
}
```

핵심 포인트:
- `model.BaseModel`을 임베드하여 직렬화 헬퍼 획득
- `ToMap()`은 `IModel` 인터페이스 구현 (`controller.ResponseItem()`에서 필요)
- `provider.Hash().Encode()`로 ID를 인코딩하여 실제 데이터베이스 ID를 숨김
- `BeforeSave()` GORM 훅으로 비밀번호를 자동 bcrypt 해싱

### 6단계: 요청 유효성 검사 구조체 생성

Gin의 binding 태그로 입력 유효성 검사를 수행합니다.

`request/user_store_request.go` 생성:

```go
package request

type UserStoreRequest struct {
	Username string `binding:"required" form:"username" json:"username"`
	Nickname string `binding:"required" form:"nickname" json:"nickname"`
	Password string `binding:"required,min=8,max=24" form:"password" json:"password"`
}
```

`request/user_update_request.go` 생성:

```go
package request

type UserUpdateRequest struct {
	Nickname string `form:"nickname" json:"nickname"`
	Password string `binding:"omitempty,min=8,max=24" form:"password" json:"password"`
	Avatar   string `form:"avatar" json:"avatar"`
}
```

`request/auth_password_request.go` 생성:

```go
package request

type AuthPasswordRequest struct {
	Username string `binding:"required" form:"username" json:"username"`
	Password string `binding:"required" form:"password" json:"password"`
}
```

### 7단계: 프로바이더 생성

프로바이더는 비즈니스 로직을 캡슐화합니다. Auth 프로바이더는 cetus의 JWT 기능을 래핑합니다.

`provider/auth_provider.go` 생성:

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

// CreateToken은 지정된 사용자의 JWT 액세스 토큰 + 리프레시 토큰을 생성
func (p *AuthProvider) CreateToken(userId uint64) (*jwt.AccessToken, error) {
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return nil, err
	}
	return guard.CreateToken(CetusProvider.Hash().Encode(userId), false)
}

// GetTokenByPassword는 사용자명/비밀번호로 인증하고 토큰을 반환
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

// AttemptAccessToken은 JWT 토큰을 검증하고 디코딩된 사용자 ID를 반환
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

// DeleteAccessToken은 토큰을 폐기 (로그아웃)
func (p *AuthProvider) DeleteAccessToken(accessToken string) error {
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return err
	}
	return guard.DeleteCredential(accessToken)
}
```

`provider/gin_toolkit.go` 생성 (라우트 파라미터 추출 유틸리티, 선택 사항):

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

### 8단계: 미들웨어 생성

`middleware/auth.go` 생성 — JWT 인증 미들웨어:

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

동작 흐름:
1. `Authorization: Bearer <token>` 헤더에서 JWT 추출
2. `AuthProvider`로 토큰 검증
3. 디코딩된 `user_id`를 Gin 컨텍스트에 저장 (하위 핸들러에서 사용)
4. 토큰이 없거나 유효하지 않으면 401 반환

### 9단계: 컨트롤러 생성

`controller/user_controller.go` 생성:

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

`controller/auth_controller.go` 생성:

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

### 10단계: 데이터베이스 마이그레이션 생성

`db/migrate.go` 생성:

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

데이터베이스 생성 및 마이그레이션 실행:

```bash
# PostgreSQL
createdb cetus

# MySQL
# mysql -u root -p -e "CREATE DATABASE cetus"

go run db/migrate.go
```

### 11단계: 엔트리 포인트 생성

`main.go` 생성:

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

	corsConf := cors.DefaultConfig()
	corsConf.AllowAllOrigins = true
	corsConf.AllowHeaders = []string{"Authorization", "Content-Type", "X-Requested-With"}
	corsConf.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	router.Use(cors.New(corsConf))

	router.Use(middleware.RequestId())

	// --- 공개 라우트 (인증 불필요) ---
	router.GET("/probe", probe.Show)
	router.POST("/users", userCtr.Store)
	router.POST("/auth/password", authCtr.AuthByPassword)

	// --- 보호된 라우트 (JWT 필수) ---
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

### 12단계: 실행!

```bash
go run main.go
```

서버가 `http://localhost:9001`에서 시작됩니다.

---

## 빠른 실행

처음부터 구축하지 않고 바로 실행하려면:

```bash
git clone https://github.com/JackDPro/cetus-demo.git
cd cetus-demo
go mod tidy
cp .env.example .env                     # .env 편집, DB/Redis 정보 설정
cd storage && sh jwt_key.sh && cd ..     # JWT 키 생성
go run storage/optimus_gen.go 104393867  # ID 해싱 파라미터 생성, .env에 복사
createdb cetus                           # 데이터베이스 생성
go run db/migrate.go                     # 마이그레이션 실행
go run main.go                           # 서버 시작
```

## API 레퍼런스

### 헬스 체크

```bash
curl http://localhost:9001/probe
```

### 사용자 등록

```bash
curl -X POST http://localhost:9001/users \
  -H "Content-Type: application/json" \
  -d '{"username": "jack", "nickname": "Jack", "password": "12345678"}'
```

응답 `201 Created`:
```json
{"id": 1580030173}
```

### 로그인

```bash
curl -X POST http://localhost:9001/auth/password \
  -H "Content-Type: application/json" \
  -d '{"username": "jack", "password": "12345678"}'
```

응답:
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

### 현재 사용자 조회

```bash
curl http://localhost:9001/users/me \
  -H "Authorization: Bearer <access_token>"
```

### ID로 사용자 조회

```bash
curl http://localhost:9001/users/1580030173 \
  -H "Authorization: Bearer <access_token>"
```

### 사용자 수정

```bash
curl -X PUT http://localhost:9001/users/1580030173 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"nickname": "Jack Pro"}'
```

### 로그아웃

```bash
curl -X POST http://localhost:9001/auth/logout \
  -H "Authorization: Bearer <access_token>"
```

## 프로젝트 구조

```
cetus-demo/
├── main.go                  # 엔트리 포인트 & 라우터 설정
├── controller/
│   ├── user_controller.go   # 사용자 CRUD 핸들러
│   └── auth_controller.go   # 로그인 / 로그아웃 핸들러
├── model/
│   └── user.go              # 사용자 모델 (BaseModel + GORM 훅)
├── middleware/
│   ├── auth.go              # JWT 인증 미들웨어
│   └── localization.go      # Accept-Language 미들웨어
├── provider/
│   ├── auth_provider.go     # JWT 토큰 작업
│   └── gin_toolkit.go       # 요청 파라미터 헬퍼
├── request/
│   ├── user_store_request.go
│   ├── user_update_request.go
│   └── auth_password_request.go
├── db/
│   └── migrate.go           # 데이터베이스 자동 마이그레이션
└── storage/
    ├── jwt_key.sh           # JWT 키 생성 스크립트
    ├── optimus_gen.go       # ID 해싱 파라미터 생성기
    ├── jwt8-der.key         # RSA 개인 키 (생성됨)
    └── jwt.pub              # RSA 공개 키 (생성됨)
```

## 사용된 Cetus 기능

| 기능 | 위치 | 설명 |
|------|------|------|
| 설정 관리 | `main.go` | `config.GetAppConfig()`, `config.GetApiConfig()` |
| 데이터베이스 | `controller/user_controller.go` | `provider.GetOrm().Db`로 GORM 쿼리 |
| JWT | `provider/auth_provider.go` | `jwt.GetJwtGuard()`로 토큰 생성/검증/폐기 |
| 비밀번호 해싱 | `model/user.go` | `BeforeSave()` 훅에서 `provider.HashMake()` |
| ID 난독화 | `model/user.go` | `provider.Hash().Encode/Decode()` |
| 요청 ID | `main.go` | `middleware.RequestId()`로 모든 요청에 추적 ID 부여 |
| 응답 헬퍼 | 모든 컨트롤러 | `ResponseItem()`, `ResponseCreated()`, `ResponseSuccess()` 등 |
| 로깅 | `main.go` | `provider.GetLogger()` 구조화된 로깅 |
| BaseModel | `model/user.go` | `ToMap()` json 태그 기반 직렬화 |

## 라이선스

MIT