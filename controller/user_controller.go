package controller

import (
	"cetus-demo/model"
	AppProvider "cetus-demo/provider"
	"cetus-demo/request"

	"github.com/JackDPro/cetus/controller"
	"github.com/JackDPro/cetus/provider"
	"github.com/gin-gonic/gin"
)

type UserController struct {
}

func NewUserController() *UserController {
	return &UserController{}
}

func (ctr *UserController) Me(c *gin.Context) {
	var user model.User
	userId, ok := c.Get("user_id")
	if !ok {
		controller.ResponseUnauthorized(c)
		return
	}
	provider.GetOrm().Db.Where("user_id", userId).First(&user)
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
	provider.GetOrm().Db.Where("user_id", decodeId).First(&user)
	if user.Id == 0 {
		controller.ResponseNotFound(c, "user not found")
		return
	}
	controller.ResponseItem(c, &user)
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

func (ctr *UserController) Update(c *gin.Context) {
	userId, err := AppProvider.GetIdFromGin[uint64](c, AppProvider.ConvertToUInt64)
	if err != nil {
		controller.ResponseUnprocessable(c, 1, "invalid id", err)
		return
	}
	var payload = &request.UserUpdateRequest{}
	if err := c.ShouldBind(payload); err != nil {
		controller.ResponseUnprocessable(c, 1, "params is invalid", err)
		return
	}
	var user model.User
	decodeId := provider.Hash().Decode(userId)
	provider.GetOrm().Db.Where("user_id", decodeId).First(&user)
	if user.Id == 0 {
		controller.ResponseNotFound(c, "user not found")
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
	controller.ResponseItem(c, &user)
}
