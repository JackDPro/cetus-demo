package provider

import (
	"cetus-demo/model"
	"fmt"

	"github.com/JackDPro/cetus/jwt"
	CetusProvider "github.com/JackDPro/cetus/provider"
)

type AuthProvider struct {
}

func NewAuthProvider() *AuthProvider {
	return &AuthProvider{}
}

// CreateToken
//
//	@Description: 创建 AccessToken
//	@receiver p
//	@param userId
//	@return *jwt.AccessToken
//	@return error
func (p *AuthProvider) CreateToken(userId uint64) (*jwt.AccessToken, error) {
	// 创建 token
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return nil, err
	}
	token, err := guard.CreateToken(CetusProvider.Hash().Encode(userId), false)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// GetTokenByPassword
//
//	@Description: 通过用户名+密码获取 AccessToken
//	@receiver p
//	@param username
//	@param password
//	@return *jwt.AccessToken
//	@return error
func (p *AuthProvider) GetTokenByPassword(username string, password string) (*jwt.AccessToken, error) {
	// 查找 user
	var item = &model.User{}
	CetusProvider.GetOrm().Db.Where("username=?", username).First(&item)
	if item.Id == 0 {
		return nil, fmt.Errorf("not found user")
	}
	_, err := p.AttemptUsernamePassword(username, password)
	if err != nil {
		return nil, fmt.Errorf("verify user failed: %v", err)
	}
	return p.CreateToken(item.Id)
}

// GetTokenByRefreshToken 通过 RefreshToken 换取 AccessToken
//
//	@Description:
//	@receiver p
//	@param refreshToken
//	@return *jwt.AccessToken
//	@return error
func (p *AuthProvider) GetTokenByRefreshToken(refreshToken string) (*jwt.AccessToken, error) {
	// 创建 token
	userId, err := p.AttemptAccessToken(refreshToken)
	if err != nil {
		return nil, err
	}
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return nil, err
	}
	return guard.CreateToken(userId, false)
}

// AttemptUsernamePassword
//
//	@Description: 通过 Username/Password 换取 AccessToken
//	@receiver p
//	@param username
//	@param password
//	@return uint64
//	@return error
func (p *AuthProvider) AttemptUsernamePassword(username string, password string) (uint64, error) {
	var item = &model.User{}
	CetusProvider.GetOrm().Db.Where("username=?", username).First(&item)
	if item.Id == 0 {
		return 0, fmt.Errorf("not found user")
	}
	if err := CetusProvider.HashCheck(password, item.Password); err != nil {
		return 0, fmt.Errorf("invalid password")
	}
	return item.Id, nil
}

// AttemptAccessToken
//
//	@Description:  验证 AccessToken 是否合法
//	@receiver p
//	@param credentials
//	@return uint64
//	@return error
func (p *AuthProvider) AttemptAccessToken(accessToken string) (uint64, error) {
	// 创建 token
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

// DeleteAccessToken
//
//	@Description: 删除 AccessToken
//	@receiver p
//	@param credentials
//	@return error
func (p *AuthProvider) DeleteAccessToken(accessToken string) error {
	guard, err := jwt.GetJwtGuard()
	if err != nil {
		return err
	}
	return guard.DeleteCredential(accessToken)
}
