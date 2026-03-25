package controller

import (
	AppProvider "cetus-demo/provider"
	"cetus-demo/request"
	"strings"

	"github.com/JackDPro/cetus/controller"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
}

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
	err := p.DeleteAccessToken(tokenArr[1])
	if err != nil {
		controller.ResponseInternalError(c, 1000, "logout failed", err)
		return
	}
	controller.ResponseSuccess(c)
}
