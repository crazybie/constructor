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
LoadAndConstruct(&r.Data, tableCsv, r)

```
## Usage

converter1(args...) | converter2(args...) | converter3(args...) | converter4(args...)...
<br> 类似unix管道，上个函数的输出用|作为下个函数的输入，然后级联下去。

- 输入"1,2,3"给split(,)
  - 结果["1", "2", "3"]
- 输入["1", "2", "3"]给map(int)
  - 结果[1,2,3]
- 输入"1,2,3"给split(,,int)
  - 结果[1,2,3]
  - 效果相当于split(,)|map(int)
- 输入"1:11,2:22"给split(,)|map(split(:,int))
  - 结果[[1,11], [2,22]]
- 输入[{a:1,b:2}, {a:11,b:22}]给dict(a)
  - 结果{1:{a:1,b:2}, 11:{a:11,b:22}}
- 输入[{a:1,b:2}, {a:1,b:22}]给group(a)
  - 结果{ 1:[{a:1,b:2}, {a:1,b:22}] }
- 输入[{a:1,b:2}, {a:1,b:22}]给group(a,dict(b))
  - 结果{ 1:{ 2: {a:1,b:2}, 22:{a:1,b:22} } }

Please check the unit tests for usage.

## All supported converters

- from(field) 
<br>从单个指定字段提取值
    - input: none
    - output: field value
- from(field, [field]...)
<br>从多个字段依次提取值生成数组
    - input: none
    - output: slice of field values
- split(sep)
<br>将输入字符串分割成字符串数组
    - input: string
    - output: slice of string
- split(sep, fn)
  <br> 将输入字符串分割成字符串数组，再对每个元素用fn进行转换
  - input: string
  - output: slice of value converted by fn
- select(idx)
<br>选择输入数组的第idx个元素
  - input: slice
  - output: slice element at idx 
- map(fn)
  <br>将输入数组的每个元素用fn进行转换，生成新的数组
    - input: slice 
    - output: slice of return value of fn
- dict(key_field)
  <br>将输入的对象数组按照指定字段的值最为键，对象作为值，生成字典
    - input: slice of struct pointer
    - output: dict with key_field as key and struct pointer as value    
- dict(key_fn, val_fn)
  <br>将输入的对象数组的每个元素传给key_fn生成key，再传给val_fn生成val，用key，val填入字典
    - input: slice of slice
    - output: sub slice as input of key convertor and result as key, same to val.
- obj(type)
  <br>用type创建一个对象，将输入的数组的每个元素依次赋值给这个对象的每个字段。
    - input: slice
    - output: instance of type with fields assigned from slice elements
- obj(type, [field,]...)
  <br>用type创建一个对象，将输入的数组的每个元素赋值给这个对象的字段，赋值顺序根据field参数指定。
  - input: slice
  - output: instance of type with fields assigned from slice elements
- group(field)
  <br>对对象数组的每个元素，以字段值为键，相同键的对象组成数组作为值，生成字典
    - input: slice of struct
    - output: dict of slice of struct, with field as key
- group(field, reduce)
  <br>类似group(field)，只是值数组会输入到reduce函数生成结果，来作为新的值
    - input: slice of struct
    - output: dict of slice of struct, with field as key and slice reduced by reduce converter
- sort(field) / sort(field, desc)
  <br>按字段对对象数组进行排序，desc表示降序，默认升序。
    - input: slice of struct
    - output: slice sorted by field
    
## Performance
- Benchmark result:
```
cpu: AMD Ryzen 5 2500U with Radeon Vega Mobile Gfx  
Benchmark_LoadAndConstruct
Benchmark_LoadAndConstruct-8   	   13798	     81496 ns/op	   13768 B/op	     443 allocs/op
Benchmark_LoadManually
Benchmark_LoadManually-8       	   24094	     49473 ns/op	   10229 B/op	     157 allocs/op
```

## Other related tools:
- Much more powerfull, general and heavy. https://github.com/alecthomas/participle
- A complete EBNF parser generator. https://github.com/goccmack/gocc
