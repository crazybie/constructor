# Constructor
A tiny tool to make data-parsing and constructing deadly easy.

Just write a few tags and no more loading code anymore.  A complex example:

```go
type LevelReward struct {
	MinLevel int32
	RewardId int64
}

type RewardCfg struct {
	ID      int64
	Mode    int32
	Value   int32
	Reward  int32
	Reward2 string         // 1:50001,11:50001,16:50001,21:50001,23:50001
	Rewards []*LevelReward `cvt:"from(Reward2)|split(,)|map(split(:,int32)|obj(LevelReward))|sort(MinLevel)"`
}

type RewardCfgs struct {
	Data []*RewardCfg
	Dict map[int64]*RewardCfg           `cvt:"from(Data)|dict(ID)"`
	Mode map[int32]map[int64]*RewardCfg `cvt:"from(Data)|group(Mode, dict(ID))"`
}


var tableCsv = `
ID,Mode,Value,Reward,Reward2,DESC
1,1,1,50001,"11:50002,1:50001,16:50003,21:50004,23:50005","第一档奖励"
2,1,3,50002,"1:50002,11:50002,16:50002,21:50002,23:50002","第二档奖励"
3,1,6,50003,"1:50003,11:50003,16:50003,21:50003,23:50003","第三档奖励"`


r := &RewardCfgs{}
LoadAndConstruct(r, &r.Data, tableCsv)

```
please check the unit tests for usage.

## All supported converters

Data flow:
<intput> converter(args...) <ouput> | <next converter>...

- from(field)
    - input: none
    - output: field value
- from(field, [field]...)
    - input: none
    - output: slice of field values
- split(sep)
    - input: string
    - output: slice of string
- split(sep, fn)
  - input: string
  - output: slice of value converted by fn
- select(idx)
  - input: slice
  - output: slice element at idx 
- map(fn)
    - input: slice 
    - output: slice of return value of fn
- dict(key_field)
    - input: slice of struct pointer
    - output: dict with key_field as key and struct pointer as value    
- dict(key_field, val_fn)
    - input: slice of slice
    - output: sub slice as input of key convertor and result as key, same to val.
- obj(type) / obj(type, [field,]...)
    - input: slice
    - output: instance of type with fields assigned from slice elements
- group(field)
    - input: slice of struct
    - output: dict of slice of struct, with field as key
- group(field, reduce)
    - input: slice of struct
    - output: dict of slice of struct, with field as key and slice reduced by reduce converter
- sort(field) / sort(field, desc)
    - input: slice of struct
    - output: slice sorted by field
    
## Performance
- Benchmark result:
```
cpu: AMD Ryzen 5 2500U with Radeon Vega Mobile Gfx  
Benchmark_LoadAndConstruct
Benchmark_LoadAndConstruct-8   	   12820	     91264 ns/op	   13883 B/op	     455 allocs/op
Benchmark_LoadManually
Benchmark_LoadManually-8       	   24999	     48401 ns/op	   10231 B/op	     157 allocs/op
```
