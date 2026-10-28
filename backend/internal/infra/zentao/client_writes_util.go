package zentao

import "time"

// timeNow 返回当前时间，包一层便于测试时注入。
func timeNow() time.Time { return time.Now() }
