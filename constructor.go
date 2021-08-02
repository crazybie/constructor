//
// Copyright (C) 2020-2021 crazybie@git.com.
//
// Constructor
//
// tiny tool to make data-parsing and construction deadly easy.
//

package constructor

import (
	"fmt"
	"github.com/gocarina/gocsv"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
)

type context struct {
	obj       reflect.Value
	fieldType reflect.Type
}

type converter func(data reflect.Value, ctx *context) reflect.Value
type ConverterCreator func([]interface{}) converter

var ConverterFactory = map[string]ConverterCreator{}

var buildInTypes = map[string]reflect.Type{
	"int":     reflect.TypeOf(0),
	"int32":   reflect.TypeOf(int32(0)),
	"int64":   reflect.TypeOf(int64(0)),
	"float32": reflect.TypeOf(float32(0)),
	"float64": reflect.TypeOf(float64(0)),
}

func init() {
	registerBuildInConverters()
}

func findType(t string, tp reflect.Type) reflect.Type {
	for tp.Kind() == reflect.Ptr {
		tp = tp.Elem()
	}
	switch tp.Kind() {
	case reflect.Map, reflect.Slice:
		return findType(t, tp.Elem())
	default:
		if tp.Name() == t {
			return tp
		}
	}
	panic(fmt.Sprintf("type not found: %v", t))
}

func panicIf(failed bool, msg string, a ...interface{}) {
	if failed {
		panic(fmt.Errorf(msg, a...))
	}
}

func tokenize(input string) []string {
	tokens := []string{}
	s := 0
	i := 0
	for ; i < len(input); i++ {
		r := input[i]
		if r == '(' || r == ')' || r == '|' || r == ',' || r == ' ' {
			if s != i {
				tokens = append(tokens, input[s:i])
			}
			if r != ' ' {
				tokens = append(tokens, string(r))
			}
			s = i + 1
		}
	}
	if s != i {
		tokens = append(tokens, input[s:i])
	}
	tokens = append(tokens, "<EOF>")
	return tokens
}

// expr: call *{ '|' call }
// arg: ident | expr
// call: ident '(' arg *{',' arg} ')'
func parseConverter(input string) converter {
	tokens := tokenize(input)
	cur := 0
	var call func() converter

	expr := func() converter {
		r := []interface{}{}
		if c := call(); c != nil {
			r = append(r, c)
		}
		for tokens[cur] == "|" {
			cur++
			r = append(r, call())
		}
		if len(r) == 0 {
			return nil
		}
		return newConverter("sequence", r)
	}
	arg := func() interface{} {
		if c := expr(); c != nil {
			return c
		}
		r := tokens[cur]
		cur++
		return r
	}
	call = func() converter {
		fnName := tokens[cur]
		if cur+1 < len(tokens) && tokens[cur+1] == "(" {
			cur += 2
			args := []interface{}{}
			for tokens[cur] != ")" {
				args = append(args, arg())
				if tokens[cur] == "," {
					cur++
				}
			}
			cur++
			fn := newConverter(fnName, args)
			panicIf(fn == nil, "invalid convertor: %v", fnName)
			return fn
		}
		if fn := newConverter(fnName, nil); fn != nil {
			cur++
			return fn
		}
		return nil
	}
	return expr()
}

func registerNumberConverter(funName string) {
	ConverterFactory[funName] = func(args []interface{}) converter {
		return func(valueStr reflect.Value, ctx *context) (ret reflect.Value) {
			v, err := strconv.ParseFloat(valueStr.String(), 64)
			panicIf(err != nil, "failed to convert %v to %v", valueStr.String(), funName)
			return reflect.ValueOf(v).Convert(buildInTypes[funName])
		}
	}
}

func registerBuildInConverters() {
	registerNumberConverter(`int`)
	registerNumberConverter(`int32`)
	registerNumberConverter(`int64`)
	registerNumberConverter(`float32`)
	registerNumberConverter(`float64`)

	ConverterFactory[`sequence`] = func(args []interface{}) converter {
		return func(rows reflect.Value, ctx *context) reflect.Value {
			v := rows
			for _, op := range args {
				if v = op.(converter)(v, ctx); !v.IsValid() {
					break
				}
			}
			return v
		}
	}

	ConverterFactory[`from`] = func(args []interface{}) converter {
		switch {
		case len(args) == 1:
			return func(_ reflect.Value, ctx *context) (ret reflect.Value) {
				ret = ctx.obj.FieldByName(args[0].(string))
				panicIf(!ret.IsValid(), "invalid filed name: %v", args[0])
				return
			}
		case len(args) > 1:
			return func(_ reflect.Value, ctx *context) (ret reflect.Value) {
				for idx, arg := range args {
					v := ctx.obj.FieldByName(arg.(string))
					panicIf(!v.IsValid(), "invalid filed name: %v", arg)
					if !ret.IsValid() {
						ret = reflect.MakeSlice(reflect.SliceOf(v.Type()), len(args), len(args))
					}
					ret.Index(idx).Set(v)
				}
				return
			}
		}
		return nil
	}

	ConverterFactory[`split`] = func(args []interface{}) converter {
		switch len(args) {
		case 1:
			return func(s reflect.Value, ctx *context) (ret reflect.Value) {
				if s.String() != "" {
					strs := strings.Split(s.String(), args[0].(string))
					ret = reflect.ValueOf(strs)
				}
				return
			}
		case 2:
			return func(s reflect.Value, ctx *context) (ret reflect.Value) {
				op := args[1].(converter)
				if s.String() != "" {
					strs := strings.Split(s.String(), args[0].(string))
					for idx, s := range strs {
						v := op(reflect.ValueOf(s), ctx)
						if !ret.IsValid() {
							ret = reflect.MakeSlice(reflect.SliceOf(v.Type()), len(strs), len(strs))
						}
						ret.Index(idx).Set(v)
					}
				}
				return
			}
		}
		return nil
	}

	ConverterFactory[`map`] = func(args []interface{}) converter {
		return func(rows reflect.Value, ctx *context) (ret reflect.Value) {
			op := args[0].(converter)
			for i := 0; i < rows.Len(); i++ {
				v := op(rows.Index(i), ctx)
				if !ret.IsValid() {
					ret = reflect.MakeSlice(reflect.SliceOf(v.Type()), rows.Len(), rows.Len())
				}
				ret.Index(i).Set(v)
			}
			return
		}
	}

	ConverterFactory[`select`] = func(args []interface{}) converter {
		idx, err := strconv.Atoi(args[0].(string))
		panicIf(err != nil, "not number: %v", args[0])
		return func(row reflect.Value, ctx *context) (ret reflect.Value) {
			return row.Index(idx)
		}
	}

	ConverterFactory[`dict`] = func(args []interface{}) converter {
		switch args[0].(type) {
		case string:
			return func(rows reflect.Value, ctx *context) (ret reflect.Value) {
				field, ok := rows.Type().Elem().Elem().FieldByName(args[0].(string))
				panicIf(!ok, "invalid field: %v", args[0])
				for i := 0; i < rows.Len(); i++ {
					src := rows.Index(i)
					key := src.Elem().Field(field.Index[0])
					if !ret.IsValid() {
						ret = reflect.MakeMapWithSize(reflect.MapOf(key.Type(), src.Type()), rows.Len())
					}
					ret.SetMapIndex(key, src)
				}
				return
			}
		case converter:
			return func(rows reflect.Value, ctx *context) (ret reflect.Value) {
				for i := 0; i < rows.Len(); i++ {
					src := rows.Index(i)
					key := args[0].(converter)(src, ctx)
					val := args[1].(converter)(src, ctx)
					if !ret.IsValid() {
						ret = reflect.MakeMapWithSize(reflect.MapOf(key.Type(), val.Type()), rows.Len())
					}
					ret.SetMapIndex(key, val)
				}
				return
			}
		}
		return nil
	}

	ConverterFactory[`obj`] = func(args []interface{}) converter {
		switch {
		case len(args) == 1:
			return func(row reflect.Value, ctx *context) (ret reflect.Value) {
				ret = reflect.New(findType(args[0].(string), ctx.fieldType))
				for i := 0; i < row.Len(); i++ {
					src := row.Index(i)
					dst := ret.Elem().Field(i)
					dst.Set(src.Convert(dst.Type()))
				}
				return
			}
		case len(args) > 1:
			return func(row reflect.Value, ctx *context) (ret reflect.Value) {
				ret = reflect.New(findType(args[0].(string), ctx.fieldType))
				for i := 0; i < row.Len(); i++ {
					src := row.Index(i)
					if i+1 < len(args) {
						dst := ret.Elem().FieldByName(args[1+i].(string))
						dst.Set(src.Convert(dst.Type()))
					}
				}
				return
			}
		}
		return nil
	}

	ConverterFactory[`group`] = func(args []interface{}) converter {
		switch len(args) {
		case 1:
			return func(rows reflect.Value, ctx *context) (ret reflect.Value) {
				field, ok := rows.Type().Elem().Elem().FieldByName(args[0].(string))
				panicIf(!ok, "invalid field %v", args[0])

				for i := 0; i < rows.Len(); i++ {
					src := rows.Index(i)
					key := src.Elem().Field(field.Index[0])
					if !ret.IsValid() {
						ret = reflect.MakeMap(reflect.MapOf(key.Type(), reflect.SliceOf(src.Type())))
					}
					v := ret.MapIndex(key)
					if !v.IsValid() {
						v = reflect.MakeSlice(reflect.SliceOf(src.Type()), 0, 0)
					}
					ret.SetMapIndex(key, reflect.Append(v, src))
				}
				return
			}
		case 2:
			return func(rows reflect.Value, ctx *context) (ret reflect.Value) {
				tmp := newConverter(`group`, args[:1])(rows, ctx)
				op := args[1].(converter)
				for _, k := range tmp.MapKeys() {
					src := tmp.MapIndex(k)
					val := op(src, ctx)
					if !ret.IsValid() {
						ret = reflect.MakeMap(reflect.MapOf(k.Type(), val.Type()))
					}
					ret.SetMapIndex(k, val)
				}
				return ret
			}
		}
		return nil
	}

	ConverterFactory[`sort`] = func(args []interface{}) converter {
		return func(rows reflect.Value, ctx *context) (ret reflect.Value) {
			field, ok := rows.Type().Elem().Elem().FieldByName(args[0].(string))
			fIdx := field.Index[0]
			panicIf(!ok, "field not found: %v", args[0])

			f64Type := buildInTypes["float64"]
			compareValues := make([]float64, 0, rows.Len())
			for i := 0; i < rows.Len(); i++ {
				compareValues = append(compareValues, rows.Index(i).Elem().Field(fIdx).Convert(f64Type).Float())
			}

			asc := true
			if len(args) > 1 && args[1].(string) == "desc" {
				asc = false
			}
			sort.Slice(rows.Interface(), func(i, j int) bool {
				if asc {
					return compareValues[i] < compareValues[j]
				} else {
					return compareValues[i] > compareValues[j]
				}
			})
			return rows
		}
	}
}

func newConverter(funName string, args []interface{}) converter {
	if c, ok := ConverterFactory[funName]; ok {
		return c(args)
	}
	return nil
}

var typeConverterCache = make(map[reflect.Type]map[string]converter)

func parseFields(objType reflect.Type) (ret map[string]converter) {
	if cache, ok := typeConverterCache[objType]; ok {
		return cache
	}
	ret = make(map[string]converter)
	for i := 0; i < objType.NumField(); i++ {
		fieldName := objType.Field(i).Name
		f := objType.Field(i).Tag.Get("cvt")
		if f != "" {
			ret[fieldName] = parseConverter(f)
		}
	}
	typeConverterCache[objType] = ret
	return ret
}

func constructObj(objValue reflect.Value) {
	panicIf(objValue.Kind() != reflect.Struct, "expect struct")

	objType := objValue.Type()
	ctx := &context{obj: objValue}
	converters := parseFields(objType)
	for i := 0; i < objType.NumField(); i++ {
		fieldInfo := objType.Field(i)
		f := objValue.Field(i)
		if c, ok := converters[fieldInfo.Name]; ok {
			ctx.fieldType = fieldInfo.Type
			v := c(f, ctx)
			if v.IsValid() {
				f.Set(v)
			}
		}
	}
}

func constructSlice(sliceValue reflect.Value) {
	panicIf(sliceValue.Kind() != reflect.Slice, "expect slice")
	for i := 0; i < sliceValue.Len(); i++ {
		objValue := sliceValue.Index(i).Elem()
		constructObj(objValue)
	}
}

func Construct(ptr interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("construct object failed: %v, stack: %s", r, debug.Stack())
		}
	}()

	v := reflect.ValueOf(ptr)
	panicIf(v.Kind() != reflect.Ptr, "expect pointer")

	switch v.Elem().Kind() {
	case reflect.Slice:
		constructSlice(v.Elem())
	case reflect.Struct:
		constructObj(v.Elem())
	}
	return nil
}

func UnmarshalStringToSlice(slicePtr interface{}, csv string) error {
	sliceValue := reflect.ValueOf(slicePtr)
	panicIf(sliceValue.Kind() != reflect.Ptr, "expect pointer to slice")
	rows := sliceValue.Elem()
	panicIf(rows.Kind() != reflect.Slice, "expect pointer to slice")
	sliceElemType := rows.Type().Elem()
	panicIf(sliceElemType.Kind() != reflect.Ptr, "expect slice of pointer")

	errChan := make(chan error)
	c := reflect.MakeChan(reflect.ChanOf(reflect.BothDir, sliceElemType.Elem()), 0)
	go func() {
		errChan <- gocsv.UnmarshalToChan(strings.NewReader(csv), c.Interface())
	}()
	data := rows
	for {
		v, notClosed := c.Recv()
		if !notClosed || v.Interface() == nil {
			break
		}
		vp := reflect.New(reflect.TypeOf(v.Interface()))
		vp.Elem().Set(v)
		data = reflect.Append(data, vp)
	}
	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
	default:
	}

	rows.Set(data)
	return nil
}

func LoadAndConstruct(objPtr interface{}, slicePtr interface{}, csv string) (retObjPtr interface{}, err error) {
	err = UnmarshalStringToSlice(slicePtr, csv)
	if err != nil {
		return nil, err
	}
	err = Construct(slicePtr)
	if err != nil {
		return nil, err
	}
	if objPtr != nil {
		err = Construct(objPtr)
		if err != nil {
			return nil, err
		}
	}
	return objPtr, nil
}
