package httpx

import "github.com/gin-gonic/gin"

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func Success(c *gin.Context, status int, data any) {
	c.JSON(status, Response{Code: 0, Message: "success", Data: data})
}

func Error(c *gin.Context, status, code int, message string, data any) {
	c.JSON(status, Response{Code: code, Message: message, Data: data})
}
