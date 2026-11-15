package zentao

import (
	"bytes"
	"encoding/json"
	"testing"
)

// UserRef 兼容禅道两种用户引用形态（对象 / 纯字符串账号）。
// 真实故障：bug-view 的 openedBy 返回字符串，详情接口 strict 解结构体
// 报 "cannot unmarshal string into Go struct field .bug.openedBy"。
func TestUserRefUnmarshal(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantAcc  string
		wantID   int
		wantReal string
	}{
		{name: "object", raw: `{"id":5855,"account":"user01","realname":"测试员"}`, wantAcc: "user01", wantID: 5855, wantReal: "测试员"},
		{name: "string", raw: `"lixiaoying"`, wantAcc: "lixiaoying"},
		{name: "empty object", raw: `{}`},
		{name: "null", raw: `null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var u UserRef
			if err := json.Unmarshal([]byte(tc.raw), &u); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.raw, err)
			}
			if u.Account != tc.wantAcc || u.ID != tc.wantID || u.Realname != tc.wantReal {
				t.Errorf("got %+v, want account=%q id=%d realname=%q", u, tc.wantAcc, tc.wantID, tc.wantReal)
			}
		})
	}
}

// Bug 详情字段兼容字符串用户引用（回归 get_bug_detail 全灭问题）。
func TestBugDetailUnmarshalWithStringUserRef(t *testing.T) {
	inner := `{"bug":{"id":437041,"title":"采集器上报","status":"active","openedBy":"lixiaoying","assignedTo":"zhangyi01"}}`
	var resp struct {
		Bug Bug `json:"bug"`
	}
	if err := json.Unmarshal([]byte(inner), &resp); err != nil {
		t.Fatalf("unmarshal bug detail: %v", err)
	}
	if resp.Bug.OpenedBy.Account != "lixiaoying" {
		t.Errorf("openedBy = %q, want lixiaoying", resp.Bug.OpenedBy.Account)
	}
	if resp.Bug.AssignedTo.Account != "zhangyi01" {
		t.Errorf("assignedTo = %q, want zhangyi01", resp.Bug.AssignedTo.Account)
	}
	if !bytes.Contains([]byte(inner), []byte("采集器")) {
		t.Fatal("sanity: test payload should contain 采集器")
	}
}
