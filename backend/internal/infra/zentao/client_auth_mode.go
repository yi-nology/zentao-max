package zentao

// 受限实例统一认证域（kydc）。
// 禅道 /user-login.html 的 JS 逻辑：
//
//	realm != "kydc" ? md5(md5(password)+rand) : password
//
// 即 kydc 域下密码明文发送（由受限实例 SSO 网关处理）。
const RealmKySSO = "kydc"

// RealmLocal 本地认证域（禅道内置账号库），密码需做 md5(md5(pw)+rand) 混淆。
const RealmLocal = "local"
