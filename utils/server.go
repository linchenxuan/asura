package utils

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

const (
	FuncIDOffset = 15 // FuncIDOffset funcid偏移.
	SetIDOffset  = 23 // SetIDOffset setid偏移.
	AreaIDOffset = 27 // AreaIDOffset areaid偏移.
)

const (
	_instIDMask = 0x00007FFF
	_funcIDMask = 0x000000FF
	_setIDMask  = 0x0000000F
	_areaIDMask = 0x0000001F
)

var (
	_srcEntityID    uint32
	_srcEntityIDStr string
	_areaID         int
	_setID          int
	_funcID         int
	_instID         int
)

// GetEntityIDByStr 将x.x.x.x的进程id解析并返回各个字段.
func GetEntityIDByStr( //nolint:revive
	entityIDStr string,
) (ientityID uint32, iareaID int, isetID int, ifuncID int, iinst int, _ error) {
	if n, eerror := fmt.Sscanf(entityIDStr, "%d.%d.%d.%d",
		&iareaID, &isetID, &ifuncID, &iinst); eerror != nil || n < 4 {
		return 0, 0, 0, 0, 0, fmt.Errorf("entityid:%s entityIDformat failed", entityIDStr)
	}
	if iareaID <= 0 || isetID < 0 || ifuncID <= 0 || iinst <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("entityid:%s entityID invalid", entityIDStr)
	}
	if iareaID != (iareaID%(_areaIDMask+1)) ||
		isetID != (isetID%(_setIDMask+1)) ||
		ifuncID != (ifuncID%(_funcIDMask+1)) ||
		iinst != (iinst%(_instIDMask+1)) {

		return 0, 0, 0, 0, 0, fmt.Errorf("entityid:%s max_entityid:%d.%d.%d.%d entityID invalid", entityIDStr, _areaIDMask, _setIDMask, _funcIDMask, _instIDMask)
	}

	tmp := 0
	tmp |= iinst
	tmp |= (ifuncID & _funcIDMask) << FuncIDOffset
	tmp |= (isetID & _setIDMask) << SetIDOffset
	tmp |= (iareaID & _areaIDMask) << AreaIDOffset

	var tmpSli [4]byte
	binary.LittleEndian.PutUint32(tmpSli[:], uint32(tmp))
	return binary.BigEndian.Uint32(tmpSli[:]), iareaID, isetID, ifuncID, iinst, nil
}

// GetStringByEntityID 根据entityID返回x.x.x.x的字符串地址.
func GetStringByEntityID(entityID uint32) string {
	var sb strings.Builder
	sb.Grow(16) //nolint:gomnd
	_, _ = sb.WriteString(strconv.Itoa(int(GetAreaIDByEntityID(entityID))))
	_, _ = sb.WriteString(".")

	_, _ = sb.WriteString(strconv.Itoa(int(GetSetIDByEntityID(entityID))))
	_, _ = sb.WriteString(".")

	_, _ = sb.WriteString(strconv.Itoa(int(GetFuncIDByEntityID(entityID))))
	_, _ = sb.WriteString(".")

	_, _ = sb.WriteString(strconv.Itoa(int(GetInstIDByEntityID(entityID))))
	return sb.String()
}

// GetAreaIDByEntityID 从entityID中解析出areaID.
func GetAreaIDByEntityID(entityID uint32) uint32 {
	entityID = ConvEndian(entityID)
	return (entityID >> AreaIDOffset) & _areaIDMask
}

// GetSetIDByEntityID 从entityID中解析出setid.
func GetSetIDByEntityID(entityID uint32) uint32 {
	entityID = ConvEndian(entityID)
	return (entityID >> SetIDOffset) & _setIDMask
}

// GetFuncIDByEntityID 从entityID中解析出funcID.
func GetFuncIDByEntityID(entityID uint32) uint32 {
	entityID = ConvEndian(entityID)
	return (entityID >> FuncIDOffset) & _funcIDMask
}

// GetInstIDByEntityID 从entityID中解析出InstID.
func GetInstIDByEntityID(entityID uint32) uint32 {
	entityID = ConvEndian(entityID)
	return entityID & _instIDMask
}

// ConvEndian 转换大小端.
func ConvEndian(entityID uint32) uint32 {
	var tmpSli [4]byte
	binary.LittleEndian.PutUint32(tmpSli[:], entityID)
	return binary.BigEndian.Uint32(tmpSli[:])
}

// SetupServerAddr 根据配置文件初始化服务器的地址相关变量.
func SetupServerAddr(entityIDStr string) error {
	var err error
	_srcEntityID, _areaID, _setID, _funcID, _instID, err = GetEntityIDByStr(entityIDStr)
	if err != nil {
		return err
	}
	_srcEntityIDStr = entityIDStr
	return nil
}

// GetEntityID 获得server的EntityID.
func GetEntityID() uint32 {
	return _srcEntityID
}

// GetEntityIDStr 获得server的EntityID的点分十进制字符串形式.
func GetEntityIDStr() string {
	return _srcEntityIDStr
}

// GetAreaID 获得server的AreaID.
func GetAreaID() int {
	return _areaID
}

// GetSetID 获得server的SetID.
func GetSetID() int {
	return _setID
}

// GetFuncID 获得server的FuncID.
func GetFuncID() int {
	return _funcID
}

// GetInsID 获得server的InsID.
func GetInsID() int {
	return _instID
}
