# constructor
A tiny tool to make data-parsing and constructing deadly easy.

Just writing a few config in the field tag and no more loading code any more.

```
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

## All supported converters:

- from(field)
- split(sep) / split(sep, converter)
- map(converter)
- dict(field)
- obj(type) / obj(type, [field,]...)
- group(field) / group(field, reduce)
- sort(field) / sort(field, desc)

## Performance tips

due to the heavy usage of reflection, it performs much worse than a hand-written loader,
so it's not a good idea to use it to handle super large tables.
