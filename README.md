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


r := &RewardCfgs{}
LoadAndConstruct(r, &r.Data, tableCsv)

```
please check the unit tests for usage.

## All supported converters:

- from(field)
- split(sep) / split(sep, converter)
- map(converter)
- dict(field)
- obj(type)
- group(field) / group(field, reduce)
- sort(field) / sort(field, desc)

## Performance tips

due to the heavy usage of reflection, it performs much bad than a hand-written loader,
so it's not a good idea to use it to handle super large tables.
