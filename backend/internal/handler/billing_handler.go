package handler

import (
	"context"
	"errors"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"time"
)

type BillingHandler struct{ service service.BillingService }

func NewBillingHandler(s service.BillingService) *BillingHandler { return &BillingHandler{s} }
func (h *BillingHandler) Plans(c *gin.Context) {
	ctx, x := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer x()
	r, e := h.service.Plans(ctx)
	if e != nil {
		h.err(c, e)
		return
	}
	httpx.Success(c, http.StatusOK, r)
}
func (h *BillingHandler) Overview(c *gin.Context) {
	ctx, x := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer x()
	r, e := h.service.Overview(ctx, tenantID(c))
	if e != nil {
		h.err(c, e)
		return
	}
	httpx.Success(c, http.StatusOK, r)
}
func (h *BillingHandler) Checkout(c *gin.Context) {
	var i struct {
		Plan string `json:"plan"`
	}
	if c.ShouldBindJSON(&i) != nil {
		httpx.Error(c, 400, 4601, "invalid plan", nil)
		return
	}
	ctx, x := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer x()
	u, e := h.service.Checkout(ctx, tenantID(c), i.Plan)
	if e != nil {
		h.err(c, e)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"url": u})
}
func (h *BillingHandler) Webhook(c *gin.Context) {
	b, e := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if e != nil {
		httpx.Error(c, 400, 4602, "invalid webhook", nil)
		return
	}
	ctx, x := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer x()
	if e = h.service.Webhook(ctx, b, c.GetHeader("Stripe-Signature")); e != nil {
		h.err(c, e)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"received": true})
}
func (h *BillingHandler) err(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrBillingUnavailable):
		httpx.Error(c, 503, 4603, "billing provider is not configured", nil)
	case errors.Is(e, service.ErrQuotaExceeded):
		httpx.Error(c, 402, 4604, "plan quota exceeded", nil)
	default:
		httpx.Error(c, 400, 4699, "billing operation failed", nil)
	}
}
