package update

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.5.0", "v0.4.1", true},
		{"0.5.0", "0.4.1", true},
		{"v0.4.1", "v0.4.1", false},
		{"v0.4.0", "v0.4.1", false},
		{"v1.0.0", "v0.9.9", true},
		{"0.10.0", "0.9.9", true}, // 数值比较而非字典序
		{"dev", "dev", false},
		{"v0.4.1", "dev", true},   // dev 构建落后于 release
		{"v0.4.1-beta", "v0.4.0", true},
	}
	for _, c := range cases {
		if got := isNewer(c.latest, c.current); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestNormalizeVer(t *testing.T) {
	if v := normalizeVer("v0.4.1"); v != [3]int{0, 4, 1} {
		t.Errorf("normalizeVer(v0.4.1) = %v", v)
	}
	if v := normalizeVer("1.2.3-beta"); v != [3]int{1, 2, 3} {
		t.Errorf("normalizeVer(1.2.3-beta) = %v", v)
	}
}
