package zorm

import (
	"sync"
	"testing"

	"github.com/modern-go/reflect2"
)

func TestBuildShapeKey(t *testing.T) {
	tests := []struct {
		name string
		op   string
		args []ZormItem
		want string
	}{
		{"empty", "Insert", nil, "caller|Insert"},
		{"fields and where", "Update", []ZormItem{Fields("name", "age"), Where(Eq("id", 1))}, "caller|Update|1`name`,`age`|8 where `id`=?"},
		{"join and limit", "Select", []ZormItem{LeftJoin("teams", "teams.id = users.team_id"), Limit(10)}, "caller|Select|2 LEFT JOIN `teams` ON teams.id = users.team_id|128 limit ?"},
		{"raw condition", "Delete", []ZormItem{Where("id = ?", 1)}, "caller|Delete|8 where id = ?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildShapeKey("caller", tt.op, tt.args); got != tt.want {
				t.Fatalf("shape key = %q, want %q", got, tt.want)
			}
		})
	}

	key := buildShapeKey("caller", "Update", []ZormItem{Where(Eq("id", 1))})
	if got := buildShapeKey("caller", "Update", []ZormItem{Where(Eq("id", 2))}); got != key {
		t.Fatal("argument values must not change the shape key")
	}
	if got := buildShapeKey("caller", "Update", []ZormItem{Where(Gt("id", 1))}); got == key {
		t.Fatal("different operators must have different shape keys")
	}
	if key != "caller|Update|8 where `id`=?" {
		t.Fatal("reusing a builder changed a previously returned key")
	}
}

func BenchmarkBuildShapeKey(b *testing.B) {
	args := []ZormItem{Fields("name", "age"), Where(Eq("id", 1))}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if buildShapeKey("caller", "Update", args) == "" {
			b.Fatal("empty shape key")
		}
	}
}

func TestStructFieldMetadataConcurrent(t *testing.T) {
	type model struct {
		ID   int64  `zorm:"id,auto_incr"`
		Name string `zorm:"name"`
	}
	rt := reflect2.TypeOf(model{}).(reflect2.StructType)
	_structFieldCache.Delete(rt.Type1())
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(useNames bool) {
			defer wg.Done()
			tbl := Table(nil, "metadata")
			tbl.Cfg.UseNameWhenTagEmpty = useNames
			<-start
			fields := tbl.getStructFieldSet(rt)
			if fields.DefaultSQL != "`id`,`name`" || fields.FieldMap["name"].Name() != "Name" {
				t.Errorf("unexpected field metadata: %+v", fields)
			}
			if fields.AutoIncrement == nil || fields.AutoIncrement.Name() != "ID" {
				t.Error("auto-increment metadata missing")
			}
		}(i%2 == 0)
	}
	close(start)
	wg.Wait()
}
