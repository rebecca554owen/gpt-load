package channel

import (
	"context"
	"gpt-load/internal/models"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// ChannelProxy 定义不同 API 通道代理的接口
type ChannelProxy interface {
	// BuildUpstreamURL 构建上游服务的目标 URL
	BuildUpstreamURL(originalURL *url.URL, groupName string) (string, error)

	// BuildUpstreamURLForAggregate 为聚合组的子组构建目标 URL
	// 它使用验证端点而非请求路径，以确保与不同上游端点的兼容性
	BuildUpstreamURLForAggregate(originalURL *url.URL, groupName string) (string, error)

	// IsConfigStale 检查通道的配置是否相对于提供的组已过期
	IsConfigStale(group *models.Group) bool

	// GetHTTPClient 返回用于标准请求的客户端
	GetHTTPClient() *http.Client

	// GetStreamClient 返回用于流式请求的客户端
	GetStreamClient() *http.Client

	// ModifyRequest 允许通道添加特定的头或修改请求
	ModifyRequest(req *http.Request, apiKey *models.APIKey, group *models.Group)

	// IsStreamRequest 检查请求是否为流式响应
	IsStreamRequest(c *gin.Context, bodyBytes []byte) bool

	// ExtractModel 从请求中提取模型名称
	ExtractModel(c *gin.Context, bodyBytes []byte) string

	// ValidateKey 检查给定的 API 密钥是否有效
	// model 参数是可选的，如果提供则覆盖通道的 TestModel
	ValidateKey(ctx context.Context, apiKey *models.APIKey, group *models.Group, model string) (bool, error)

	// ApplyModelRedirect 根据组的重定向规则应用模型重定向
	ApplyModelRedirect(req *http.Request, bodyBytes []byte, group *models.Group) ([]byte, error)

	// TransformModelList 根据重定向规则转换模型列表响应
	TransformModelList(req *http.Request, bodyBytes []byte, group *models.Group) (map[string]any, error)

	// GetChannelType 返回通道类型标识符
	GetChannelType() string
}
