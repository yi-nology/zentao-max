package handlers

import (
	"context"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/pkg/errors"
	"github.com/yi-nology/zentao-max/backend/internal/service"
)

type UserHandler struct {
	userService UserServicer
}

func NewUserHandler(userService UserServicer) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) GetUsers(ctx context.Context, c *app.RequestContext) {
	page := 1
	pageSize := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := c.Query("pageSize"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	result, err := h.userService.GetUsers(page, pageSize)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, result)
}

func (h *UserHandler) GetUsersAll(ctx context.Context, c *app.RequestContext) {
	users, err := h.userService.GetUsersAll()
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

func (h *UserHandler) GetCurrentUser(ctx context.Context, c *app.RequestContext) {
	user, err := h.userService.GetCurrentUser()
	if err != nil {
		if _, ok := err.(*service.ValidationError); ok {
			errors.InternalError(c, err.Error())
			return
		}
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, map[string]interface{}{
		"user": user,
	})
}
