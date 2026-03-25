package provider

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/JackDPro/cetus/provider"
	"github.com/gin-gonic/gin"
)

func GetUserIdFromGin[T any](c *gin.Context, userKey string) (T, error) {
	var userId T
	userIdAny, ok := c.Get(userKey)
	if !ok {
		return userId, errors.New("middle not have user_id")
	}
	userId, ok = userIdAny.(T)
	if !ok {
		return userId, errors.New("convert user_id to uint64 failed")
	}
	return userId, nil
}

func GetOriginIdFromGin(c *gin.Context) (uint64, error) {
	idStr := c.Param("id")
	idNum, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, err
	}
	return provider.Hash().Decode(uint64(idNum)), nil
}

func GetIdFromGin[T any](c *gin.Context, convertFunc func(string) (T, error)) (T, error) {
	idStr := c.Param("id")
	result, err := convertFunc(idStr)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("failed to convert id to target type: %w", err)
	}
	return result, nil
}

func ConvertToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func ConvertToUInt64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

func ConvertToFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func ConvertToString(s string) (string, error) {
	return s, nil
}
