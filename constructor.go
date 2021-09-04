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
	"sort"
	"strconv"
	"strings"
)

type Context struct {
	obj       reflect.Value
	fieldType reflect.Type
}

type Converter func(data reflect.Value, ctx *Context) reflect.Value
type ConverterCreator func([]interface{}) Converter

var converterFactory = map[string]ConverterCreator{}

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
	panic(fmt.Errorf("type not found: %v", t))
}

func panicIf(failed bool, msg string, a ...interface{}) {
	if failed {
		panic(fmt.Errorf(msg, a...))
	}
}

func tokenize(input string) []string {
	var tokens []string
	s := 0
	i := 0
	for ; i < len(input); i++ {
		r := input[i]
		if r == '(' || r == ')' || r == '|' || r == ',' || r == ' ' || r == '!' {
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

// simp_expr: call | !call
// expr: simp_expr *{ '|' simp_expr }
// arg: ident | expr
// call: ident '(' arg *{',' arg} ')'
func parseConverter(input string) Converter {
	tokens := tokenize(input)
	cur := 0
	var call func() Converter

	simpleExpr := func() Converter {
		if tokens[cur] == "!" {
			cur++
			return newConverter("not", []interface{}{call()})
		}
		return call()
	}

	expr := func() Converter {
		var r []interface{}
		if c := simpleExpr(); c != nil {
			r = append(r, c)
		}
		for tokens[cur] == "|" {
			cur++
			r = append(r, simpleExpr())
		}
		if len(r) == 0 {
			return nil
		} else if len(r) == 1 {
			return r[0].(Converter)
		} else {
			return newConverter("sequence", r)
		}
	}
	arg := func() interface{} {
		if c := expr(); c != nil {
			return c
		}
		r := tokens[cur]
		cur++
		return r
	}
	call = func() Converter {
		fnName := tokens[cur]
		if cur+1 < len(tokens) && tokens[cur+1] == "(" {
			cur += 2
			var args []interface{}
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

func registerIntConverter(funName string, bitSz int) {
	converterFactory[funName] = func(args []interface{}) Converter {
		return func(valueStr reflect.Value, ctx *Context) (ret reflect.Value) {
			v, err := strconv.ParseInt(valueStr.String(), 10, bitSz)
			panicIf(err != nil, "failed to convert %v to %v", valueStr.String(), funName)
			return reflect.ValueOf(v).Convert(buildInTypes[funName])
		}
	}
}

func registerFloatConverter(funName string, bitSz int) {
	converterFactory[funName] = func(args []interface{}) Converter {
		return func(valueStr reflect.Value, ctx *Context) (ret reflect.Value) {
			v, err := strconv.ParseFloat(valueStr.String(), bitSz)
			panicIf(err != nil, "failed to convert %v to %v", valueStr.String(), funName)
			return reflect.ValueOf(v).Convert(buildInTypes[funName])
		}
	}
}
func RegisterFn(name string, fn ConverterCreator) {
	converterFactory[name] = fn
}

func RegisterNormalFn(name string, fn interface{}) {
	converterFactory[name] = func(args []interface{}) Converter {
		fnVal := reflect.ValueOf(fn)
		return func(data reflect.Value, ctx *Context) reflect.Value {
			fnArgs := []reflect.Value{data}
			for _, arg := range args {
				fnArgs = append(fnArgs, reflect.ValueOf(arg))
			}
			out := fnVal.Call(fnArgs)
			if len(out) > 0 {
				return out[0]
			}
			return reflect.Value{}
		}
	}
}

func registerBuildInConverters() {
	registerIntConverter(`int`, 32)
	registerIntConverter(`int32`, 32)
	registerIntConverter(`int64`, 64)
	registerFloatConverter(`float32`, 32)
	registerFloatConverter(`float64`, 64)

	converterFactory[`not`] = func(args []interface{}) Converter {
		return func(rows reflect.Value, ctx *Context) reflect.Value {
			op := args[0].(Converter)
			r := op(rows, ctx)
			return reflect.ValueOf(!r.Bool())
		}
	}

	converterFactory[`sequence`] = func(args []interface{}) Converter {
		return func(rows reflect.Value, ctx *Context) reflect.Value {
			v := rows
			for _, op := range args {
				if v = op.(Converter)(v, ctx); !v.IsValid() {
					break
				}
			}
			return v
		}
	}

	converterFactory[`from`] = func(args []interface{}) Converter {
		switch {
		case len(args) == 1:
			return func(_ reflect.Value, ctx *Context) (ret reflect.Value) {
				ret = ctx.obj.FieldByName(args[0].(string))
				panicIf(!ret.IsValid(), "invalid filed name: %v", args[0])
				return
			}
		case len(args) > 1:
			return func(_ reflect.Value, ctx *Context) (ret reflect.Value) {
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

	RegisterFn("zero", func([]interface{}) Converter {
		return func(value reflect.Value, ctx *Context) reflect.Value {
			return reflect.ValueOf(value.IsZero())
		}
	})

	converterFactory["filter"] = func(args []interface{}) Converter {
		return func(slice reflect.Value, ctx *Context) reflect.Value {
			ret := reflect.MakeSlice(slice.Type(), 0, slice.Cap())
			predicate := args[0].(Converter)
			for i := 0; i < slice.Len(); i++ {
				if predicate(slice.Index(i), ctx).Bool() {
					ret = reflect.Append(ret, slice.Index(i))
				}
			}
			return ret
		}
	}

	converterFactory[`split`] = func(args []interface{}) Converter {
		switch len(args) {
		case 1:
			return func(s reflect.Value, ctx *Context) (ret reflect.Value) {
				if s.String() != "" {
					strs := strings.Split(s.String(), args[0].(string))
					ret = reflect.ValueOf(strs)
				}
				return
			}
		case 2:
			return func(s reflect.Value, ctx *Context) (ret reflect.Value) {
				op := args[1].(Converter)
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

	converterFactory[`map`] = func(args []interface{}) Converter {
		return func(rows reflect.Value, ctx *Context) (ret reflect.Value) {
			op := args[0].(Converter)
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

	converterFactory[`select`] = func(args []interface{}) Converter {
		idx, err := strconv.Atoi(args[0].(string))
		panicIf(err != nil, "not number: %v", args[0])
		return func(row reflect.Value, ctx *Context) (ret reflect.Value) {
			return row.Index(idx)
		}
	}

	converterFactory[`dict`] = func(args []interface{}) Converter {
		switch args[0].(type) {
		case string:
			return func(rows reflect.Value, ctx *Context) (ret reflect.Value) {
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
		case Converter:
			return func(rows reflect.Value, ctx *Context) (ret reflect.Value) {
				for i := 0; i < rows.Len(); i++ {
					src := rows.Index(i)
					key := args[0].(Converter)(src, ctx)
					val := args[1].(Converter)(src, ctx)
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

	converterFactory[`obj`] = func(args []interface{}) Converter {
		switch {
		case len(args) == 1:
			return func(row reflect.Value, ctx *Context) (ret reflect.Value) {
				ret = reflect.New(findType(args[0].(string), ctx.fieldType))
				for i := 0; i < row.Len(); i++ {
					src := row.Index(i)
					dst := ret.Elem().Field(i)
					dst.Set(src.Convert(dst.Type()))
				}
				return
			}
		case len(args) > 1:
			return func(row reflect.Value, ctx *Context) (ret reflect.Value) {
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

	converterFactory[`group`] = func(args []interface{}) Converter {
		switch len(args) {
		case 1:
			return func(rows reflect.Value, ctx *Context) (ret reflect.Value) {
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
			return func(rows reflect.Value, ctx *Context) (ret reflect.Value) {
				tmp := newConverter(`group`, args[:1])(rows, ctx)
				op := args[1].(Converter)
				for iter := tmp.MapRange(); iter.Next(); {
					k := iter.Key()
					src := iter.Value()
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

	converterFactory[`sort`] = func(args []interface{}) Converter {
		return func(rows reflect.Value, ctx *Context) (ret reflect.Value) {
			var elemType reflect.Type
			var compareValues reflect.Value

			if len(args) > 0 && args[0].(string) != "_" {
				field, ok := rows.Type().Elem().Elem().FieldByName(args[0].(string))
				elemType = field.Type
				fIdx := field.Index[0]
				panicIf(!ok, "field not found: %v", args[0])

				compareValues = reflect.MakeSlice(reflect.SliceOf(elemType), rows.Len(), rows.Len())
				for i := 0; i < rows.Len(); i++ {
					compareValues.Index(i).Set(rows.Index(i).Elem().Field(fIdx))
				}
			} else {
				elemType = rows.Type().Elem()
				compareValues = rows
			}

			asc := true
			if len(args) > 1 && args[1].(string) == "desc" {
				asc = false
			}
			var cmp func(i, j int) bool
			switch elemType.Kind() {
			case reflect.String:
				cmp = func(i, j int) bool {
					if asc {
						return compareValues.Index(i).String() < compareValues.Index(j).String()
					} else {
						return compareValues.Index(i).String() > compareValues.Index(j).String()
					}
				}
			case reflect.Bool, reflect.Int, reflect.Int32, reflect.Int16, reflect.Int64:
				cmp = func(i, j int) bool {
					if asc {
						return compareValues.Index(i).Int() < compareValues.Index(j).Int()
					} else {
						return compareValues.Index(i).Int() > compareValues.Index(j).Int()
					}
				}
			case reflect.Float64, reflect.Float32:
				cmp = func(i, j int) bool {
					if asc {
						return compareValues.Index(i).Float() < compareValues.Index(j).Float()
					} else {
						return compareValues.Index(i).Float() > compareValues.Index(j).Float()
					}
				}
			default:
				panic(fmt.Errorf("sort: don't know how to order type: %v", elemType.Name()))
			}
			sort.Slice(rows.Interface(), cmp)
			return rows
		}
	}
}

func newConverter(funName string, args []interface{}) Converter {
	if c, ok := converterFactory[funName]; ok {
		return c(args)
	}
	return nil
}

var typeConverterCache = make(map[reflect.Type]map[string]Converter)

func parseFields(objType reflect.Type) (ret map[string]Converter) {
	if cache, ok := typeConverterCache[objType]; ok {
		return cache
	}
	ret = make(map[string]Converter)
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
	converters := parseFields(objType)
	ctx := &Context{obj: objValue}

	fieldsCnt := objType.NumField()
	for i := 0; i < fieldsCnt; i++ {
		fieldInfo := objType.Field(i)
		if c, ok := converters[fieldInfo.Name]; ok {
			f := objValue.Field(i)
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

func Construct(ptr interface{}) {
	v := reflect.ValueOf(ptr)
	panicIf(v.Kind() != reflect.Ptr, "expect pointer")

	switch v.Elem().Kind() {
	case reflect.Slice:
		constructSlice(v.Elem())
	case reflect.Struct:
		constructObj(v.Elem())
	}
}

func UnmarshalStringToSlice(slicePtr interface{}, csv string) error {
	slicePtrValue := reflect.ValueOf(slicePtr)
	panicIf(slicePtrValue.Kind() != reflect.Ptr, "expect pointer to slice")
	slice := slicePtrValue.Elem()
	panicIf(slice.Kind() != reflect.Slice, "expect pointer to slice")
	sliceElemType := slice.Type().Elem()
	panicIf(sliceElemType.Kind() != reflect.Ptr, "expect slice of pointer")

	errChan := make(chan error)
	c := reflect.MakeChan(reflect.ChanOf(reflect.BothDir, sliceElemType), 0)
	go func() {
		errChan <- gocsv.UnmarshalToChan(strings.NewReader(csv), c.Interface())
	}()
	for {
		v, notClosed := c.Recv()
		if !notClosed || v.Interface() == nil {
			break
		}
		slice = reflect.Append(slice, v)
	}
	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
	default:
	}

	slicePtrValue.Elem().Set(slice)
	return nil
}

func LoadAndConstruct(slicePtr interface{}, csv string, objPtr interface{}) (retObjPtr interface{}, err error) {
	err = UnmarshalStringToSlice(slicePtr, csv)
	if err != nil {
		return nil, err
	}
	Construct(slicePtr)
	if objPtr != nil {
		Construct(objPtr)
	}
	return objPtr, nil
}
