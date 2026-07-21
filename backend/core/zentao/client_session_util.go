package zentao

import (
	"crypto/md5"
	"encoding/hex"
)

// md5Hex 返回字符串的 MD5 十六进制摘要（小写）。用于 local 域密码混淆：
// 禅道 /user-login.html JS 在 realm != "kydc" 时发送 md5(md5(password) + rand)。
func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
