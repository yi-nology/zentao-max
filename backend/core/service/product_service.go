package service

import (
	"github.com/yi-nology/common/biz/zentao"

	"github.com/yi-nology/zentao-max/backend/core/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/core/zentao"
)

// ProductService 产品业务逻辑服务
// 负责处理产品相关的业务逻辑
type ProductService struct {
	client *myzentao.Client
}

// NewProductService 创建产品服务
func NewProductService(client *myzentao.Client) *ProductService {
	return &ProductService{client: client}
}

// GetProducts 获取产品列表
// 业务逻辑：获取所有产品并转换为VO
func (s *ProductService) GetProducts() ([]vo.ProductVO, error) {
	products, err := s.client.GetProducts()
	if err != nil {
		return nil, err
	}

	// 转换为VO
	return s.convertToVO(products), nil
}

// convertToVO 将zentao.Product转换为vo.ProductVO
func (s *ProductService) convertToVO(products []zentao.Product) []vo.ProductVO {
	if len(products) == 0 {
		return []vo.ProductVO{}
	}

	result := make([]vo.ProductVO, 0, len(products))
	for _, product := range products {
		result = append(result, vo.ProductVO{
			ID:     product.ID,
			Name:   product.Name,
			Code:   product.Code,
			Type:   product.Type,
			Status: product.Status,
			Desc:   product.Desc,
		})
	}
	return result
}
