package zentao

import (
	"context"
	"fmt"

	"github.com/yi-nology/common/biz/zentao"
)

// GetProducts 获取产品列表
func (c *Client) GetProducts() ([]zentao.Product, error) {
	return c.getProductsSession(context.Background())
}

// GetProduct 获取产品详情
func (c *Client) GetProduct(productID int) (*zentao.Product, error) {
	all, err := c.getProductsSession(context.Background())
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == productID {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("产品 %d 不存在", productID)
}

// GetProductContext 获取产品详情（支持 context 取消）
func (c *Client) GetProductContext(ctx context.Context, productID int) (*zentao.Product, error) {
	all, err := c.getProductsSession(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == productID {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("产品 %d 不存在", productID)
}
