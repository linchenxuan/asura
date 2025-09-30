package net

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 测试RouteStrategy的String方法
func TestRouteStrategy_String(t *testing.T) {
	// 测试所有的路由策略类型
	assert.Equal(t, "P2P", RouteP2P.String())
	assert.Equal(t, "Random", RouteRand.String())
	assert.Equal(t, "Hash", RouteHash.String())
	assert.Equal(t, "Broadcast", RouteBroadCast.String())
	assert.Equal(t, "Multicast", RouteMulCast.String())
	assert.Equal(t, "Internal", RouteInter.String())
	// 测试未知的路由策略类型
	assert.Equal(t, "Unknown", RouteStrategy(999).String())
}

// 测试RouteTypeP2P的功能
func TestRouteTypeP2P(t *testing.T) {
	// 测试创建P2P路由
	entityID := uint64(123456)
	route := NewRouteTypeP2P(entityID)
	assert.NotNil(t, route)
	assert.Equal(t, entityID, route.DstEntityID)
	assert.Equal(t, fmt.Sprintf("p2p:%d", entityID), route.routeKey)

	// 测试GetRouteKey方法
	assert.Equal(t, fmt.Sprintf("p2p:%d", entityID), route.GetRouteKey())

	// 测试GetRouteStrategy方法
	assert.Equal(t, RouteP2P, route.GetRouteStrategy())

	// 测试Validate方法
	err := route.Validate()
	assert.NoError(t, err)

	// 测试String方法
	// 注意：这个测试假设RouteTypeP2P实现了String方法
	// 如果没有实现，可能需要添加或者跳过这个测试
	// assert.Contains(t, route.String(), fmt.Sprintf("%d", entityID))
}

// 测试路由接口的通用行为
func TestRouteInterface(t *testing.T) {
	// 创建不同类型的路由
	p2pRoute := NewRouteTypeP2P(123)

	// 测试P2P路由接口方法
	assert.Equal(t, "p2p:123", p2pRoute.GetRouteKey())
	assert.Equal(t, RouteP2P, p2pRoute.GetRouteStrategy())
	assert.NoError(t, p2pRoute.Validate())

	// 测试路由接口的扩展性
	// 创建一个自定义的路由实现，测试接口的灵活性
	customRoute := &CustomRoute{}
	assert.NotEmpty(t, customRoute.GetRouteKey())
	assert.NotEqual(t, RouteStrategy(0), customRoute.GetRouteStrategy())
	assert.NoError(t, customRoute.Validate())
}

// 测试多个P2P路由实例的唯一性
func TestMultipleP2PRoutes(t *testing.T) {
	// 创建两个不同实体ID的P2P路由
	route1 := NewRouteTypeP2P(100)
	route2 := NewRouteTypeP2P(200)

	// 验证它们的路由键是不同的
	assert.NotEqual(t, route1.GetRouteKey(), route2.GetRouteKey())
	assert.NotEqual(t, route1.GetRouteKey(), fmt.Sprintf("p2p:%d", 200))
	assert.NotEqual(t, route2.GetRouteKey(), fmt.Sprintf("p2p:%d", 100))

	// 验证它们的路由策略相同
	assert.Equal(t, route1.GetRouteStrategy(), route2.GetRouteStrategy())
	assert.Equal(t, RouteP2P, route1.GetRouteStrategy())
}

// 自定义路由实现，用于测试接口的灵活性
// 注意：这只是为了测试目的，实际使用中应该使用框架提供的路由类型

type CustomRoute struct{}

func (r *CustomRoute) GetRouteKey() string {
	return "custom:route"
}

func (r *CustomRoute) GetRouteStrategy() RouteStrategy {
	return RouteStrategy(99)
}

func (r *CustomRoute) Validate() error {
	return nil
}

func (r *CustomRoute) String() string {
	return "CustomRoute"
}

// 测试路由键的一致性
func TestRouteKeyConsistency(t *testing.T) {
	// 测试多次创建相同实体ID的路由，确保路由键一致
	const entityID = uint64(456)
	route1 := NewRouteTypeP2P(entityID)
	route2 := NewRouteTypeP2P(entityID)

	assert.Equal(t, route1.GetRouteKey(), route2.GetRouteKey())
	assert.Equal(t, fmt.Sprintf("p2p:%d", entityID), route1.GetRouteKey())
	assert.Equal(t, fmt.Sprintf("p2p:%d", entityID), route2.GetRouteKey())
}
