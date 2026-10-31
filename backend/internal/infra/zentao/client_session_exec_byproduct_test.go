package zentao

import (
	"testing"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// 场景：executionsByProductSession 的产品→项目→执行过滤（纯函数部分）。
// 回归背景：原实现无视 productID 返回全实例执行（2726 个），dashboard 任务统计
// 被拖到 90s 预算掐断且混入其它产品的数据（产品 1029 显示全实例 tasks 3957）。
func TestFilterExecutionContexts(t *testing.T) {
	allExecs := []zentaosdk.Execution{
		{ID: 101, Project: 3193, Name: "产品1029的执行A"},
		{ID: 102, Project: 3194, Name: "产品1029的执行B"},
		{ID: 201, Project: 8888, Name: "其它产品的执行"},
		{ID: 202, Project: 0, Name: "无主执行"},
	}
	projects := []zentaosdk.Project{
		{ID: 3193, Name: "服务器研发部预研项目"},
		{ID: 3194, Name: "另一个关联项目"},
	}

	got := filterExecutionContexts(allExecs, projects)
	if len(got) != 2 {
		t.Fatalf("want 2 executions of product 1029, got %d: %+v", len(got), got)
	}
	if got[0].Exec.ID != 101 || got[0].ProjName != "服务器研发部预研项目" || got[0].ExecName != "产品1029的执行A" {
		t.Errorf("第一条组装错误: %+v", got[0])
	}
	if got[1].Exec.ID != 102 || got[1].ProjName != "另一个关联项目" {
		t.Errorf("第二条组装错误: %+v", got[1])
	}

	// 空项目集（如产品无关联项目）→ 空结果，而不是退化为全量
	if got := filterExecutionContexts(allExecs, nil); len(got) != 0 {
		t.Fatalf("空项目集应返回空列表，got %d", len(got))
	}
}
