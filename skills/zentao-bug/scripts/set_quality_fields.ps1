# Set fixedPackageVersion + reasonAnalysis + advice + implementation + testAdvise in one bug-edit
# map: bid|fpv|ra_quote|ad_quote|impl_quote|ta_quote (pure ASCII, python quote_plus)
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
[System.Net.ServicePointManager]::ServerCertificateValidationCallback = {$true}
$cookies = New-Object System.Net.CookieContainer
Add-Type -AssemblyName System.Web.Extensions
$js = New-Object System.Web.Script.Serialization.JavaScriptSerializer
$js.MaxJsonLength = 104857600
function Get-Url($url,$t){ if(-not $t){$t=45000}
  try{$req=[System.Net.HttpWebRequest]::Create($url);$req.CookieContainer=$cookies;$req.Timeout=$t;$req.ReadWriteTimeout=$t;$req.Proxy=$null
    $resp=$req.GetResponse();$sr=New-Object System.IO.StreamReader($resp.GetResponseStream(),[System.Text.Encoding]::UTF8)
    $c=$sr.ReadToEnd();$sr.Close();$resp.Close();return $c}catch{return "ERR:"+$_.Exception.Message}}
function Post-Url($url,$body){
  try{$req=[System.Net.HttpWebRequest]::Create($url);$req.CookieContainer=$cookies;$req.Method="POST"
    $req.ContentType="application/x-www-form-urlencoded";$req.Timeout=30000;$req.ReadWriteTimeout=30000;$req.Proxy=$null
    $bytes=[System.Text.Encoding]::UTF8.GetBytes($body);$req.ContentLength=$bytes.Length
    $st=$req.GetRequestStream();$st.Write($bytes,0,$bytes.Length);$st.Close()
    $resp=$req.GetResponse();$sr=New-Object System.IO.StreamReader($resp.GetResponseStream(),[System.Text.Encoding]::UTF8)
    $c=$sr.ReadToEnd();$sr.Close();$resp.Close();return $c}catch{return "POSTERR:"+$_.Exception.Message}}
function Post-UrlUtf8($url,$bodyStr){
  $req=[System.Net.HttpWebRequest]::Create($url);$req.CookieContainer=$cookies;$req.Method="POST"
  $req.ContentType="application/x-www-form-urlencoded; charset=utf-8";$req.Timeout=30000;$req.ReadWriteTimeout=30000;$req.Proxy=$null
  $bytes=[System.Text.Encoding]::UTF8.GetBytes($bodyStr);$req.ContentLength=$bytes.Length
  $st=$req.GetRequestStream();$st.Write($bytes,0,$bytes.Length);$st.Close()
  try{$resp=$req.GetResponse();$sr=New-Object System.IO.StreamReader($resp.GetResponseStream(),[System.Text.Encoding]::UTF8);$c=$sr.ReadToEnd();$sr.Close();$resp.Close();return $c}
  catch{return "ERR:"+$_.Exception.Message}
}
$USERNAME="";$PASS="";$BASE="https://zentao.example.com";$REALM=""
foreach($l in (Get-Content "C:\Users\user\zt_creds.txt")){
  if($l -match '^ZT_USERNAME=(.*)$'){$USERNAME=$Matches[1]}
  if($l -match '^ZT_PASSWORD=(.*)$'){$PASS=$Matches[1]}
  if($l -match '^ZT_BASE=(.*)$'){$BASE=$Matches[1]}
  if($l -match '^ZT_REALM=(.*)$'){$REALM=$Matches[1]}
}
$utf8=New-Object System.Text.UTF8Encoding($false)
$null=Get-Url "$BASE/user-login-Lw==.html"
$rnd=(Get-Url "$BASE/user-refreshRandom.html").Replace('ERR:','').Trim()
$body="account=$([uri]::EscapeDataString($USERNAME))&password=$([uri]::EscapeDataString($PASS))&verifyRand=$rnd&keepLogin=0&referer=%2F"
if($REALM -ne ""){$body=$body+"&realm=$([uri]::EscapeDataString($REALM))"}
$null=Post-Url "$BASE/user-login.json" $body
$lines=[System.IO.File]::ReadAllLines("C:\Users\user\all_map.txt")
[System.IO.File]::WriteAllText("C:\Users\user\all_out.txt","START`r`n",$utf8)
foreach($l in $lines){
  $p=$l.Split('|')
  $bid=$p[0];$fpv=$p[1];$encRA=$p[2];$encAD=$p[3];$encImpl=$p[4];$encTA=$p[5]
  $openedBuild="";$execution="";$title=""
  try{$bj=Get-Url "$BASE/bug-view-$bid.json" 60000;$o=$js.DeserializeObject($bj);$in=$js.DeserializeObject($o["data"]);$bug=$in["bug"];$openedBuild=$bug.Item("openedBuild");$execution=$bug.Item("execution");$title=$bug.Item("title")}catch{}
  $title=$title -replace '&#039;',"'"
  $editBody="openedBuild=$openedBuild&execution=$execution&title=$title&fixedPackageVersion=$fpv&reasonAnalysis=$encRA&advice=$encAD&implementation=$encImpl&testAdvise=$encTA"
  $r=Post-UrlUtf8 "$BASE/bug-edit-$bid.json" $editBody
  $ok=$r -match '"status":"success"'
  Start-Sleep -Seconds 1
  $fpvA="";$raA="";$adA="";$implA="";$taA=""
  try{$bj2=Get-Url "$BASE/bug-view-$bid.json" 60000;$o2=$js.DeserializeObject($bj2);$in2=$js.DeserializeObject($o2["data"]);$bug2=$in2["bug"];$fpvA=$bug2.Item("fixedPackageVersion");$raA=$bug2.Item("reasonAnalysis");$adA=$bug2.Item("advice");$implA=$bug2.Item("implementation");$taA=$bug2.Item("testAdvise")}catch{}
  $fpvM=if($fpvA -eq $fpv){"FPV_OK"}else{"FPV_BAD"}
  $raM=if($raA -ne "" -and $raA -ne $null){"RA_OK"}else{"RA_EMPTY"}
  $adM=if($adA -ne "" -and $adA -ne $null){"AD_OK"}else{"AD_EMPTY"}
  $implM=if($implA -ne "" -and $implA -ne $null){"IMPL_OK"}else{"IMPL_EMPTY"}
  $taM=if($taA -ne "" -and $taA -ne $null){"TA_OK"}else{"TA_EMPTY"}
  $line="BUG $bid edit=$ok $fpvM $raM $adM $implM $taM"
  [System.IO.File]::AppendAllText("C:\Users\user\all_out.txt",$line+"`r`n",$utf8)
}
Write-Output "DONE"
