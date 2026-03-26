# Cetus Demo

[English](README.md) | [中文](README_zh.md) | **日本語** | [한국어](README_ko.md)

[Cetus](https://github.com/JackDPro/cetus) を使って完全な REST API をゼロから構築するステップバイステップチュートリアル。ユーザー登録、JWT 認証、ユーザー CRUD 操作を実装しています。

## 前提条件

- Go 1.21+
- PostgreSQL（または MySQL）
- Redis
- OpenSSL

## プロジェクトの取得

```bash
git clone https://github.com/JackDPro/cetus-demo.git
cd cetus-demo
go mod tidy
```

## ステップバイステップガイド

このガイドではプロジェクトをゼロから構築します。すぐに実行したい場合は[クイックラン](#クイックラン)へ。

---

### ステップ 1：プロジェクトの初期化

```bash
mkdir cetus-demo && cd cetus-demo
go mod init cetus-demo
go get github.com/JackDPro/cetus
go get github.com/gin-contrib/cors
```

ディレクトリ構造を作成：

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

### ステップ 2：環境設定

プロジェクトルートに `.env` ファイルを作成：

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

> `OPTIMUS_*` と `JWT_*` の値はプレースホルダーです。次の2ステップで実際の値を生成します。

### ステップ 3：JWT 鍵の生成

Cetus JWT には **PKCS#8 DER** 形式の秘密鍵と **PEM** 形式の公開鍵が必要です。

`storage/jwt_key.sh` を作成：

```bash
#!/bin/sh
# RSA 秘密鍵を生成（PKCS#1 PEM 形式）
openssl genrsa -out jwt1.pem 2048

# PKCS#8 DER 形式に変換（cetus が要求する形式）
openssl pkcs8 -topk8 -inform PEM -outform DER \
  -in jwt1.pem -out jwt8-der.key -nocrypt

# 公開鍵をエクスポート（PEM 形式）
openssl rsa -in jwt1.pem -pubout -out jwt.pub
```

実行：

```bash
cd storage && sh jwt_key.sh && cd ..
```

生成されるファイル：

| ファイル | 形式 | 用途 |
|----------|------|------|
| `storage/jwt8-der.key` | PKCS#8 DER | トークン署名（秘密鍵） |
| `storage/jwt.pub` | PEM | トークン検証（公開鍵） |

### ステップ 4：ID 難読化パラメータの生成

Cetus は [Optimus](https://github.com/pjebs/optimus-go) を使用して、データベースの連番 ID を推測不可能な整数にエンコードします（例：`1` -> `1580030173`）。3つの値が必要です：`OPTIMUS_PRIME`、`OPTIMUS_INVERSE`、`OPTIMUS_RANDOM`。

`storage/optimus_gen.go` を作成：

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

**使い方：**

1. http://primes.utm.edu/lists/small/millions/ にアクセスし、任意の `.txt` ファイルをダウンロードして開き、`2,147,483,647` 未満の素数を**ランダムに1つ選びます**。
2. 選んだ素数をジェネレーターに渡します：

```bash
go run storage/optimus_gen.go 104393867
```

出力：
```
OPTIMUS_PRIME=104393867
OPTIMUS_INVERSE=1990279033
OPTIMUS_RANDOM=1333095938
```

3. これらの値を `.env` ファイルにコピーし、プレースホルダーを置き換えます。

> **重要：** 本番環境にデプロイ後、これらの値を**絶対に変更しないでください**。既存のエンコード済み ID がすべて無効になります。

### ステップ 5：モデルの作成

`model/user.go` を作成：

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

// ToMap は IModel インターフェースを実装
// API レスポンス返却前にデータベース ID をエンコード
func (m *User) ToMap() (map[string]interface{}, error) {
	data, err := m.BaseModel.ToMap(m)
	if err != nil {
		return nil, err
	}
	data["id"] = provider.Hash().Encode(m.Id)
	return data, nil
}

// BeforeSave は GORM フックで、保存前にパスワードを自動ハッシュ
func (m *User) BeforeSave(_ *gorm.DB) (err error) {
	if m.Password != "" {
		m.Password, err = provider.HashMake(m.Password)
	}
	return
}
```

ポイント：
- `model.BaseModel` を埋め込んでシリアライズヘルパーを取得
- `ToMap()` は `IModel` インターフェースを実装（`controller.ResponseItem()` で必要）
- `provider.Hash().Encode()` で ID をエンコードし、生のデータベース ID を隠蔽
- `BeforeSave()` GORM フックでパスワードを自動的に bcrypt ハッシュ

### ステップ 6：リクエストバリデーション構造体の作成

Gin の binding タグで入力バリデーションを行います。

`request/user_store_request.go` を作成：

```go
package request

type UserStoreRequest struct {
	Username string `binding:"required" form:"username" json:"username"`
	Nickname string `binding:"required" form:"nickname" json:"nickname"`
	Password string `binding:"required,min=8,max=24" form:"password" json:"password"`
}
```

`request/user_update_request.go` を作成：

```go
package request

type UserUpdateRequest struct {
	Nickname string `form:"nickname" json:"nickname"`
	Password string `binding:"omitempty,min=8,max=24" form:"password" json:"password"`
	Avatar   string `form:"avatar" json:"avatar"`
}
```

`request/auth_password_request.go` を作成：

```go
package request

type AuthPasswordRequest struct {
	Username string `binding:"required" form:"username" json:"username"`
	Password string `binding:"required" form:"password" json:"password"`
}
```

### ステップ 7：プロバイダーの作成

プロバイダーはビジネスロジックをカプセル化します。Auth プロバイダーは cetus の JWT 機能をラップします。

`provider/auth_provider.go` を作成：

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

// CreateToken は指定ユーザーの JWT アクセストークン + リフレッシュトークンを作成
func (p *AuthProvider) CreateToken(userId uint64) (*jwt.AccessToken, error) {
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return nil, err
	}
	return guard.CreateToken(CetusProvider.Hash().Encode(userId), false)
}

// GetTokenByPassword はユーザー名/パスワードで認証しトークンを返す
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

// AttemptAccessToken は JWT トークンを検証しデコードされたユーザー ID を返す
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

// DeleteAccessToken はトークンを無効化（ログアウト）
func (p *AuthProvider) DeleteAccessToken(accessToken string) error {
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return err
	}
	return guard.DeleteCredential(accessToken)
}
```

`provider/gin_toolkit.go` を作成（ルートパラメータ抽出ユーティリティ、オプション）：

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

### ステップ 8：ミドルウェアの作成

`middleware/auth.go` を作成 — JWT 認証ミドルウェア：

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

動作の流れ：
1. `Authorization: Bearer <token>` ヘッダーから JWT を抽出
2. `AuthProvider` でトークンを検証
3. デコードされた `user_id` を Gin コンテキストに保存（下流ハンドラーで使用）
4. トークンが無い、または無効な場合は 401 を返す

### ステップ 9：コントローラーの作成

`controller/user_controller.go` を作成：

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

`controller/auth_controller.go` を作成：

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

### ステップ 10：データベースマイグレーションの作成

`db/migrate.go` を作成：

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

データベースを作成しマイグレーションを実行：

```bash
# PostgreSQL
createdb cetus

# MySQL
# mysql -u root -p -e "CREATE DATABASE cetus"

go run db/migrate.go
```

### ステップ 11：エントリーポイントの作成

`main.go` を作成：

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

	// --- 公開ルート（認証不要） ---
	router.GET("/probe", probe.Show)
	router.POST("/users", userCtr.Store)
	router.POST("/auth/password", authCtr.AuthByPassword)

	// --- 保護されたルート（JWT 必須） ---
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

### ステップ 12：実行！

```bash
go run main.go
```

サーバーが `http://localhost:9001` で起動します。

---

## クイックラン

ゼロから構築せず、すぐに実行したい場合：

```bash
git clone https://github.com/JackDPro/cetus-demo.git
cd cetus-demo
go mod tidy
cp .env.example .env                     # .env を編集し DB/Redis 情報を設定
cd storage && sh jwt_key.sh && cd ..     # JWT 鍵を生成
go run storage/optimus_gen.go 104393867  # ID ハッシュパラメータを生成、.env にコピー
createdb cetus                           # データベース作成
go run db/migrate.go                     # マイグレーション実行
go run main.go                           # サーバー起動
```

## API リファレンス

### ヘルスチェック

```bash
curl http://localhost:9001/probe
```

### ユーザー登録

```bash
curl -X POST http://localhost:9001/users \
  -H "Content-Type: application/json" \
  -d '{"username": "jack", "nickname": "Jack", "password": "12345678"}'
```

レスポンス `201 Created`：
```json
{"id": 1580030173}
```

### ログイン

```bash
curl -X POST http://localhost:9001/auth/password \
  -H "Content-Type: application/json" \
  -d '{"username": "jack", "password": "12345678"}'
```

レスポンス：
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

### 現在のユーザーを取得

```bash
curl http://localhost:9001/users/me \
  -H "Authorization: Bearer <access_token>"
```

### ID でユーザーを取得

```bash
curl http://localhost:9001/users/1580030173 \
  -H "Authorization: Bearer <access_token>"
```

### ユーザー更新

```bash
curl -X PUT http://localhost:9001/users/1580030173 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"nickname": "Jack Pro"}'
```

### ログアウト

```bash
curl -X POST http://localhost:9001/auth/logout \
  -H "Authorization: Bearer <access_token>"
```

## プロジェクト構造

```
cetus-demo/
├── main.go                  # エントリーポイント & ルーター設定
├── controller/
│   ├── user_controller.go   # ユーザー CRUD ハンドラー
│   └── auth_controller.go   # ログイン / ログアウトハンドラー
├── model/
│   └── user.go              # ユーザーモデル（BaseModel + GORM フック）
├── middleware/
│   ├── auth.go              # JWT 認証ミドルウェア
│   └── localization.go      # Accept-Language ミドルウェア
├── provider/
│   ├── auth_provider.go     # JWT トークン操作
│   └── gin_toolkit.go       # リクエストパラメータヘルパー
├── request/
│   ├── user_store_request.go
│   ├── user_update_request.go
│   └── auth_password_request.go
├── db/
│   └── migrate.go           # データベースオートマイグレーション
└── storage/
    ├── jwt_key.sh           # JWT 鍵生成スクリプト
    ├── optimus_gen.go       # ID ハッシュパラメータジェネレーター
    ├── jwt8-der.key         # RSA 秘密鍵（生成済み）
    └── jwt.pub              # RSA 公開鍵（生成済み）
```

## 使用している Cetus 機能

| 機能 | 場所 | 説明 |
|------|------|------|
| 設定管理 | `main.go` | `config.GetAppConfig()`、`config.GetApiConfig()` |
| データベース | `controller/user_controller.go` | `provider.GetOrm().Db` で GORM クエリ |
| JWT | `provider/auth_provider.go` | `jwt.GetJwtGuard()` でトークン作成/検証/無効化 |
| パスワードハッシュ | `model/user.go` | `BeforeSave()` フックで `provider.HashMake()` |
| ID 難読化 | `model/user.go` | `provider.Hash().Encode/Decode()` |
| リクエスト ID | `main.go` | `middleware.RequestId()` で全リクエストにトレース ID 付与 |
| レスポンスヘルパー | 全コントローラー | `ResponseItem()`、`ResponseCreated()`、`ResponseSuccess()` 等 |
| ログ | `main.go` | `provider.GetLogger()` 構造化ログ |
| BaseModel | `model/user.go` | `ToMap()` json タグによるシリアライズ |

## ライセンス

MIT