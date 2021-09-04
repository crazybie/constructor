package constructor

import (
	"github.com/gocarina/gocsv"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func Equal(t *testing.T, a, b interface{}) {
	if a != b {
		t.Errorf("%v != %v", a, b)
	}
}

type LevelReward struct {
	MinLevel int32
	RewardId int64
}

type RewardCfg struct {
	ID        int64
	Mode      int32
	Value     int32
	ModeValue []int32 `cvt:"from(Mode,Value)"`
	Reward    int32
	Reward2   string         // 1:50001,11:50001,16:50001,21:50001,23:50001
	Rewards   []*LevelReward `cvt:"from(Reward2)|split(,)|map(split(:,int32)|obj(LevelReward,MinLevel,RewardId))|sort(MinLevel)"`
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

func Test_basic(t *testing.T) {

	r := &RewardCfgs{}
	_, err := LoadAndConstruct(&r.Data, tableCsv, r)
	Equal(t, err, nil)

	Equal(t, len(r.Data), 3)

	Equal(t, r.Data[0].ModeValue[0], r.Data[0].Mode)
	Equal(t, r.Data[0].ModeValue[1], r.Data[0].Value)

	Equal(t, len(r.Data[0].Rewards), 5)
	rewards := r.Data[0].Rewards
	Equal(t, rewards[0].MinLevel, int32(1))
	Equal(t, rewards[0].RewardId, int64(50001))
	Equal(t, rewards[1].MinLevel, int32(11))
	Equal(t, rewards[1].RewardId, int64(50002))
	Equal(t, rewards[2].MinLevel, int32(16))
	Equal(t, rewards[2].RewardId, int64(50003))
	Equal(t, rewards[3].MinLevel, int32(21))
	Equal(t, rewards[3].RewardId, int64(50004))
	Equal(t, rewards[4].MinLevel, int32(23))
	Equal(t, rewards[4].RewardId, int64(50005))

	Equal(t, r.Dict[1].ID, int64(1))
	Equal(t, r.Dict[1].Value, int32(1))

	Equal(t, r.Dict[2].ID, int64(2))
	Equal(t, r.Dict[2].Value, int32(3))

	Equal(t, r.Dict[3].ID, int64(3))
	Equal(t, r.Dict[3].Value, int32(6))

	Equal(t, len(r.Mode), 1)
	Equal(t, len(r.Mode[1]), 3)
	Equal(t, r.Mode[1][1], r.Dict[1])
	Equal(t, r.Mode[1][2], r.Dict[2])
	Equal(t, r.Mode[1][3], r.Dict[3])
}

func Benchmark_LoadAndConstruct(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r := &RewardCfgs{}
		_, _ = LoadAndConstruct(&r.Data, tableCsv, r)
	}
}

func Benchmark_LoadManually(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ret := &RewardCfgs{
			Dict: make(map[int64]*RewardCfg),
			Mode: make(map[int32]map[int64]*RewardCfg),
		}
		_ = gocsv.UnmarshalStringToCallback(tableCsv, func(e RewardCfg) {
			rs := ret.Mode[e.Mode]
			if rs == nil {
				ret.Mode[e.Mode] = make(map[int64]*RewardCfg)
			}

			e.ModeValue = append(e.ModeValue, e.Mode)
			e.ModeValue = append(e.ModeValue, e.Value)

			rewards := []*LevelReward{}
			ss := strings.Split(e.Reward2, ",")
			for _, s := range ss {
				r := strings.Split(s, ":")
				level, _ := strconv.Atoi(r[0])
				rewardId, _ := strconv.Atoi(r[1])
				rewards = append(rewards, &LevelReward{
					MinLevel: int32(level),
					RewardId: int64(rewardId),
				})
			}
			sort.Slice(rewards, func(i, j int) bool {
				return rewards[i].MinLevel < rewards[j].MinLevel
			})
			e.Rewards = rewards
			v := &e
			ret.Mode[e.Mode][e.ID] = v
			ret.Dict[e.ID] = v
			ret.Data = append(ret.Data, v)
		})
	}
}

func Test_Dict2(t *testing.T) {
	type Data struct {
		Id  int32
		Kvs string
		Kv  map[string]float64 `cvt:"from(Kvs)|split(;)|map(split(:))|dict(select(0),select(1)|float64)"`
	}

	d := &Data{
		Id:  1,
		Kvs: "k1:11;k2:22;k3:33",
	}
	Construct(d)
	Equal(t, len(d.Kv), 3)
	Equal(t, d.Kv[`k1`], float64(11))
	Equal(t, d.Kv[`k2`], float64(22))
	Equal(t, d.Kv[`k3`], float64(33))
}

func Test_CustomConverter(t *testing.T) {
	type Data struct {
		Id       int32
		Str      string
		LowerStr string `cvt:"from(Str)|lower"`
	}

	d := &Data{
		Id:  1,
		Str: "ABC",
	}
	RegisterNormalFn("lower", strings.ToLower)
	Construct(d)
	Equal(t, d.LowerStr, "abc")
}

func TestConstruct_Sort(t *testing.T) {
	type D struct {
		S  []int
		S2 []int `cvt:"from(S)|sort"`
	}
	d := D{
		S: []int{1, 3, 2},
	}
	Construct(&d)
	Equal(t, d.S2[0], 1)
	Equal(t, d.S2[1], 2)
	Equal(t, d.S2[2], 3)

	type S struct {
		S  []string
		S2 []string `cvt:"from(S)|sort(_,desc)"`
	}
	s := S{
		S: []string{"a", "c", "b"},
	}
	Construct(&s)
	Equal(t, s.S2[0], "c")
	Equal(t, s.S2[1], "b")
	Equal(t, s.S2[2], "a")
}
