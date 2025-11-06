package config

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/go-viper/mapstructure/v2"
)

// Config interface defines the basic configuration contract
type Config interface {
	GetName() string
	Validate() error
}

// ConfigChangeListener interface for listening to configuration changes
// Business layers should implement this interface to handle config changes
type ConfigChangeListener interface {
	// OnConfigChanged is called when configuration changes
	OnConfigChanged(configName string, newConfig, oldConfig Config) error
}

// ConfigChangeNotifier interface for notifying configuration changes
type ConfigChangeNotifier interface {
	// AddChangeListener adds a configuration change listener
	AddChangeListener(listener ConfigChangeListener)
	// RemoveChangeListener removes a configuration change listener
	RemoveChangeListener(listener ConfigChangeListener)
	// NotifyConfigChanged notifies all listeners about configuration change
	NotifyConfigChanged(configName string, newConfig, oldConfig Config)
}

func Decode(input any, output any) error {
	config := &mapstructure.DecoderConfig{
		ErrorUnused:      true,
		Result:           output,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			requiredFieldHook(),
		),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(input)
}

func requiredFieldHook() mapstructure.DecodeHookFunc {
	return func(from reflect.Type, to reflect.Type, data any) (any, error) {
		if from.Kind() != reflect.Map || to.Kind() != reflect.Struct {
			return data, nil
		}

		dataMap, ok := data.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("requiredFieldHook: expected map[string]any, got %T", data)
		}
		toType := to

		keyMap := make(map[string]struct{}, len(dataMap))
		for key := range dataMap {
			keyMap[strings.ToLower(key)] = struct{}{}
		}
		for i := 0; i < toType.NumField(); i++ {
			field := toType.Field(i)
			tagValue := field.Tag.Get("mapstructure")

			// 匿名组合的struct跳过不处理
			if field.Type.Kind() == reflect.Struct && field.Type.NumField() == 0 {
				continue
			}

			if tagValue == "" && !isFirstLetterUpperCase(field.Name) {
				continue
			}

			// 如果没有mapstructure tag，则使用字段名
			if tagValue == "" {
				tagValue = field.Name
			}

			// 跳过squash标记的字段
			if index := strings.Index(tagValue, "squash"); index != -1 {
				continue
			}

			// 跳过omitempty标记的字段
			if index := strings.Index(tagValue, "omitempty"); index != -1 {
				continue
			}

			if _, ok := keyMap[strings.ToLower(tagValue)]; !ok {
				return nil, fmt.Errorf("required field missing: %s", tagValue)
			}
		}
		return data, nil
	}
}

func isFirstLetterUpperCase(s string) bool {
	if len(s) == 0 {
		return false
	}
	firstRune := []rune(s)[0]
	return unicode.IsUpper(firstRune)
}
