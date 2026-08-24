package domain

import "errors"

var (
	ErrInvalidInput = errors.New("输入内容无效")
	ErrInvalidState = errors.New("当前状态不允许此操作")
	ErrNotFound     = errors.New("案件不存在")
	ErrConflict     = errors.New("案件版本冲突")
	ErrDuplicate    = errors.New("幂等请求已处理")
	ErrForbidden    = errors.New("操作者无权执行此操作")
)

type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

func invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}
