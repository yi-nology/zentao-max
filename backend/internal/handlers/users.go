package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/internal/service"
	"github.com/yi-nology/zentao-max/backend/pkg/errors"
)

type UserHandler struct {
	userService UserServicer
}

func NewUserHandler(userService UserServicer) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) GetUsers(ctx context.Context, c *app.RequestContext) {
	// issue #11：此前手写解析绕过 DTO 校验，page 无上界导致
	// (page-1)*pageSize 溢出为负、切片越界 panic；对齐 bugs 等端点的 DTO 模式
	var query dto.UserQueryDTO
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}

	if err := query.Validate(); err != nil {
		errors.Error(c, err)
		return
	}

	result, err := h.userService.GetUsers(query.Page, query.PageSize)
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
