package main

import (
	"fmt"
	"reflect"
)

func i2s(data interface{}, out interface{}) error {
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("out not pointer %v", reflect.TypeOf(out))
	}
	err := process(data, rv)
	if err != nil {
		return err
	}
	return nil
}

func process(data interface{}, v reflect.Value) error {
	fmt.Printf("data %#v\n", data)
	fmt.Printf("out %#v\n", v)
	switch v.Kind() {
	case reflect.Pointer:
		v2 := v.Elem()
		switch v2.Kind() {
		case reflect.Struct:
			_, ok := data.(map[string]interface{})
			if !ok {
				return fmt.Errorf("data for struct not map %v", data)
			}
			return process(data, v2)
		case reflect.Slice:
			_, ok := data.([]interface{})
			if !ok {
				return fmt.Errorf("data for slice not list %v", data)
			}
			return process(data, v2)
		default:
			return fmt.Errorf("unknown struct under pointer %v", v.Elem())
		}
	case reflect.Slice:
		dSlice, ok := data.([]interface{})
		if !ok {
			return fmt.Errorf("data for slice not slice %v", data)
		}
		n := len(dSlice)
		vSlice := reflect.MakeSlice(v.Type(), n, n)
		for i := 0; i < n; i++ {
			err := process(dSlice[i], vSlice.Index(i))
			if err != nil {
				return err
			}
		}
		v.Set(vSlice)
		return nil
	case reflect.Struct:
		fCnt := v.NumField()
		dMap, ok := data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("data for struct not map %v", data)
		}
		v2t := v.Type()
		for i := 0; i < fCnt; i++ {
			field := v.Field(i)
			ft := v2t.Field(i)
			dataVal, ok := dMap[ft.Name]
			if ok {
				err := process(dataVal, field)
				if err != nil {
					return err
				}
			}
		}
	case reflect.Bool:
		dataBool, ok := data.(bool)
		if ok {
			v.SetBool(dataBool)
		} else {
			return fmt.Errorf("can't parse as int %v", data)
		}
	case reflect.String:
		dataStr, ok := data.(string)
		if ok {
			v.SetString(dataStr)
		} else {
			return fmt.Errorf("can't parse as int %v", data)
		}
	case reflect.Int:
		value := 0
		dataFloat, ok := data.(float64)
		if ok {
			value = int(dataFloat)
		} else {
			dataInt, ok := data.(int)
			if ok {
				value = dataInt
			} else {
				return fmt.Errorf("can't parse as int %v", data)
			}
		}
		v.SetInt(int64(value))
	default:
		fmt.Println("Error", v.Kind())
		return fmt.Errorf("unknown type %v", v.Type())
	}
	return nil
}
