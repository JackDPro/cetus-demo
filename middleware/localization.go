package middleware

import (
	"strings"

	"github.com/JackDPro/cetus/provider"
	"github.com/gin-gonic/gin"
)

func Localization() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := c.GetHeader("Accept-Language")
		if strings.Contains(lang, "en") {
			provider.GetTranslate().SetLanguage("en")
		} else {
			provider.GetTranslate().SetLanguage("zh")
		}
		c.Next()
	}
}
