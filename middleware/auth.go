package middleware

import (
	"cetus-demo/provider"
	"strings"

	"github.com/JackDPro/cetus/controller"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// before reqeust
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
		// after request
		//log.Println("request finish")
	}
}
