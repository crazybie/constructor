package constructor

import (
	"github.com/gocarina/gocsv"
	"github.com/stretchr/testify/assert"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

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
	_, err := LoadAndConstruct(r, &r.Data, tableCsv)
	assert.Zero(t, err)

	assert.Equal(t, len(r.Data), 3)

	assert.Equal(t, r.Data[0].ModeValue[0], r.Data[0].Mode)
	assert.Equal(t, r.Data[0].ModeValue[1], r.Data[0].Value)

	assert.Equal(t, len(r.Data[0].Rewards), 5)
	rewards := r.Data[0].Rewards
	assert.Equal(t, rewards[0].MinLevel, int32(1))
	assert.Equal(t, rewards[0].RewardId, int64(50001))
	assert.Equal(t, rewards[1].MinLevel, int32(11))
	assert.Equal(t, rewards[1].RewardId, int64(50002))
	assert.Equal(t, rewards[2].MinLevel, int32(16))
	assert.Equal(t, rewards[2].RewardId, int64(50003))
	assert.Equal(t, rewards[3].MinLevel, int32(21))
	assert.Equal(t, rewards[3].RewardId, int64(50004))
	assert.Equal(t, rewards[4].MinLevel, int32(23))
	assert.Equal(t, rewards[4].RewardId, int64(50005))

	assert.Equal(t, r.Dict[1].ID, int64(1))
	assert.Equal(t, r.Dict[1].Value, int32(1))

	assert.Equal(t, r.Dict[2].ID, int64(2))
	assert.Equal(t, r.Dict[2].Value, int32(3))

	assert.Equal(t, r.Dict[3].ID, int64(3))
	assert.Equal(t, r.Dict[3].Value, int32(6))

	assert.Equal(t, len(r.Mode), 1)
	assert.Equal(t, len(r.Mode[1]), 3)
	assert.Equal(t, r.Mode[1][1], r.Dict[1])
	assert.Equal(t, r.Mode[1][2], r.Dict[2])
	assert.Equal(t, r.Mode[1][3], r.Dict[3])
}

func Benchmark_LoadAndConstruct(b *testing.B) {
	r := &RewardCfgs{}
	for i := 0; i < b.N; i++ {
		_, _ = LoadAndConstruct(r, &r.Data, tableCsv)
	}
}

func Benchmark_LoadManually(b *testing.B) {
	ret := &RewardCfgs{
		Dict: make(map[int64]*RewardCfg),
		Mode: make(map[int32]map[int64]*RewardCfg),
	}
	for i := 0; i < b.N; i++ {
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
	_ = Construct(d)
	assert.Equal(t, len(d.Kv), 3)
	assert.Equal(t, d.Kv[`k1`], float64(11))
	assert.Equal(t, d.Kv[`k2`], float64(22))
	assert.Equal(t, d.Kv[`k3`], float64(33))
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
	ConverterFactory[`lower`] = func(args []interface{}) converter {
		return func(data reflect.Value, ctx *context) reflect.Value {
			return reflect.ValueOf(strings.ToLower(data.Interface().(string)))
		}
	}
	_ = Construct(d)
	assert.Equal(t, d.LowerStr, "abc")
}
