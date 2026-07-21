package zentao

import (
	"encoding/json"
	"os"
	"testing"
)

// Phase2b 实体映射单元测试（基于 testdata 脱敏样本，无需凭据）。

func TestSessionCaseMapping(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_testcase_browse.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	var wrapper struct {
		Sample []sessionCase `json:"sample"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(wrapper.Sample) == 0 {
		t.Fatal("case sample empty")
	}
	c := wrapper.Sample[0].toSDK()
	if c.ID == 0 {
		t.Error("case ID not mapped (parseCaseID 失败？)")
	}
	if c.Title == "" {
		t.Error("case Title not mapped")
	}
	t.Logf("mapped case OK: id=%d title=%q type=%s", c.ID, c.Title, c.Type)
}

func TestSessionProgramMapping(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_program_browse.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	var wrapper struct {
		Sample map[string]sessionProgram `json:"sample"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(wrapper.Sample) == 0 {
		t.Fatal("program sample empty")
	}
	for _, p := range wrapper.Sample {
		m := p.toSDK()
		if m.ID == 0 || m.Name == "" {
			t.Errorf("program mapping incomplete: %+v", m)
		}
		t.Logf("mapped program OK: id=%d name=%q type=%s", m.ID, m.Name, m.Type)
		return
	}
}

func TestSessionTestTaskMapping(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_testtask_browse.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	var wrapper struct {
		Sample map[string]sessionTestTask `json:"sample"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(wrapper.Sample) == 0 {
		t.Fatal("testtask sample empty")
	}
	for _, tt := range wrapper.Sample {
		m := tt.toSDK()
		if m.ID == 0 || m.Name == "" {
			t.Errorf("testtask mapping incomplete: %+v", m)
		}
		t.Logf("mapped testtask OK: id=%d name=%q owner=%s", m.ID, m.Name, m.Owner.Account)
		return
	}
}

// TestParseCaseID 验证 "case_2343233" 字符串 id 的解析。
func TestParseCaseID(t *testing.T) {
	cases := []struct {
		in   string // 原始 JSON
		want int
	}{
		{`"case_2343233"`, 2343233},
		{`12345`, 12345},
		{`"case_x"`, 0},
		{`""`, 0},
	}
	for _, c := range cases {
		if got := parseCaseID(json.RawMessage(c.in)); got != c.want {
			t.Errorf("parseCaseID(%q)=%d, want %d", c.in, got, c.want)
		}
	}
}

// TestDoPaginate 验证通用分页辅助。
func TestDoPaginate(t *testing.T) {
	in := []int{1, 2, 3, 4, 5, 6, 7}
	if got := doPaginate(in, 1, 3); len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Errorf("page1: %v", got)
	}
	if got := doPaginate(in, 3, 3); len(got) != 1 || got[0] != 7 {
		t.Errorf("page3: %v", got)
	}
	if got := doPaginate(in, 5, 3); got != nil {
		t.Errorf("page5 should be nil: %v", got)
	}
}
