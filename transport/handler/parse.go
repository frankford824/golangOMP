package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func parseID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

func parseInt(value string) (int, error) {
	parsed, err := strconv.ParseInt(value, 10, 32)
	return int(parsed), err
}

func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func parseBool(value string) (bool, error) {
	return strconv.ParseBool(value)
}
