package main

import (
	"cetus-demo/controller"
	AppMiddleware "cetus-demo/middleware"
	"errors"
	"fmt"

	"github.com/JackDPro/cetus/config"
	CertController "github.com/JackDPro/cetus/controller"
	"github.com/JackDPro/cetus/middleware"
	"github.com/JackDPro/cetus/provider"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var probe *CertController.ProbeController
var userCtr *controller.UserController
var authCtr *controller.AuthController

func init() {
	provider.GetLogger().Infow("init success")
	probe = CertController.NewProbeController()
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

	// 自定义验证器（保留扩展点）
	_ = binding.Validator

	startApiServer(router)

	address := fmt.Sprintf("0.0.0.0:%d", apiConf.HttpPort)
	provider.GetLogger().Infow("start api server success", "address", address)

	err := router.Run(address)
	if err != nil {
		return errors.New("start http server failed error=" + err.Error())
	}
	return nil
}

func startApiServer(router *gin.Engine) {
	//lmt := tollbooth.NewLimiter(1, nil)

	router.Use(middleware.RequestId())
	{
		// no auth router
		router.Use().GET("/probe", probe.Show)

		// register
		router.POST("/users", userCtr.Store)

		// limit rate
		//Use(middleware.LimitRate(lmt)).
		router.POST("/auth/password", authCtr.AuthByPassword)

		// auth
		authorized := router.Use(AppMiddleware.AuthMiddleware())
		authorized.POST("/auth/logout", authCtr.Logout)
		authorized.GET("/users/me", userCtr.Me)
		authorized.GET("/users/:id", userCtr.Show)
		authorized.PUT("/users/:id", userCtr.Update)
	}
}

func main() {
	// 启动 rest api
	err := StartServer()
	if err != nil {
		panic(err)
	}
}
