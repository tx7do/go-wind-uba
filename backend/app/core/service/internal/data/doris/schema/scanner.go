package schema

import (
	"encoding/json"
	"fmt"
)

type MapStringString map[string]string

func (m *MapStringString) Scan(value any) error {
	if value == nil {
		*m = nil
		return nil
	}

	// 数据库返回 []uint8 转字符串
	bs, ok := value.([]uint8)
	if !ok {
		return fmt.Errorf("expected []uint8, got %T", value)
	}

	// JSON 反序列化到 map
	return json.Unmarshal(bs, m)
}

type MapStringFloat64 map[string]float64

func (m *MapStringFloat64) Scan(value any) error {
	if value == nil {
		*m = nil
		return nil
	}

	// 数据库返回 []uint8 转字符串
	bs, ok := value.([]uint8)
	if !ok {
		return fmt.Errorf("expected []uint8, got %T", value)
	}

	// JSON 反序列化到 map
	return json.Unmarshal(bs, m)
}

type StringArray []string

func (m *StringArray) Scan(value any) error {
	if value == nil {
		*m = nil
		return nil
	}

	// 数据库返回 []uint8 转字符串
	bs, ok := value.([]uint8)
	if !ok {
		return fmt.Errorf("expected []uint8, got %T", value)
	}

	// JSON 反序列化到 map
	return json.Unmarshal(bs, m)
}
