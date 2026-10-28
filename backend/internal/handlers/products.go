package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/pkg/errors"
)

type ProductHandler struct {
	productService ProductServicer
}

func NewProductHandler(productService ProductServicer) *ProductHandler {
	return &ProductHandler{productService: productService}
}

func (h *ProductHandler) GetProducts(ctx context.Context, c *app.RequestContext) {
	result, err := h.productService.GetProducts()
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, result)
}
