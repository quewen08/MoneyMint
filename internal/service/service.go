// Package service 承载业务逻辑：参数/业务规则校验、事务编排、跨实体的聚合操作。
//
// 它只依赖 repository（数据访问）与 auth（纯函数），不感知 HTTP。
// handler 层负责解析请求、调用本层、写出响应。
package service

import (
	"familyledger/internal/repository"
)

// Service 是业务逻辑的统一入口。
type Service struct {
	repo *repository.Store
}

// New 构造 Service。
func New(repo *repository.Store) *Service {
	return &Service{repo: repo}
}
