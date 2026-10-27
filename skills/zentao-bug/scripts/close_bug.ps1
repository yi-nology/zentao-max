# Close BUG: set fixedPackageVersion + resolve (if active)
# Input map: C:\Users\user\close_bug_map.txt, each line bid|fixedPackageVersion|comment_quote_plus
#   - fixedPackageVersion format: name=version (e.g. example-pkg=1.0.0-1)
#   - comment: python urllib.parse.quote_plus to pure ASCII (PS4 can't read UTF-8 file, see pitfall #18)
# Generate map on local (python):
#   import urllib.parse
#   bugs=[("123456","example-pkg=1.0.0-1","fixed in example-pkg 1.0.0-1")]
#   with open(r'C:\Users\user\close_bug_map.txt','w',encoding='ascii') as f:
#       for bid,fpv,c in bugs: f.write(f'{bid}|{fpv}|{urllib.parse.quote_plus(c)}\n')
#   scp to jumpbox:C:/Users/user/close_bug_map.txt

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
# Chinese must be sent as UTF-8 bytes, ContentType with charset=utf-8, NEVER EscapeDataString (pitfall #12)
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

# Read map with default encoding (PS4 can't read UTF-8 file, pitfall #18; map is pure ASCII)
$lines=[System.IO.File]::ReadAllLines("C:\Users\user\close_bug_map.txt")
[System.IO.File]::WriteAllText("C:\Users\user\close_bug_out.txt","START lines="+$lines.Count+"`r`n",$utf8)
foreach($l in $lines){
  $parts=$l.Split('|')
  $bid=$parts[0].Trim()
  $fpv=$parts[1].Trim()
  $comment=$parts[2].Trim()
  # 1. bug-view get original values (bug-edit is full update, must carry openedBuild+execution+title, pitfall #13)
  $openedBuild="";$execution="";$title="";$status=""
  try{
    $bj=Get-Url "$BASE/bug-view-$bid.json" 60000
    $outer=$js.DeserializeObject($bj)
    $inner=$js.DeserializeObject($outer["data"])
    $bug=$inner["bug"]
    $openedBuild=$bug.Item("openedBuild")
    $execution=$bug.Item("execution")
    $title=$bug.Item("title")
    $status=$bug.Item("status")
  }catch{}
  # 2. title: replace &#039; (HTML entity) with ' to avoid bug-edit double-escaping & -> &amp; (pitfall #17)
  $title=$title -replace '&#039;',"'"
  # 3. bug-edit set fixedPackageVersion (full: original values + fixedPackageVersion)
  $editBody="openedBuild=$openedBuild&execution=$execution&title=$title&fixedPackageVersion=$fpv"
  $r1=Post-UrlUtf8 "$BASE/bug-edit-$bid.json" $editBody
  $editOk=$r1 -match '"status":"success"'
  # 4. bug-resolve (only if active; closed needs activate first, pitfall #14)
  $resolveOk="skip(status=$status)"
  if($status -eq "active"){
    $resolveBody="resolution=fixed&resolvedBuild=trunk&comment=$comment"
    $r2=Post-UrlUtf8 "$BASE/bug-resolve-$bid.json" $resolveBody
    $resolveOk=$(if($r2 -match '"status":"success"'){"ok"}else{$r2.Substring(0,[Math]::Min(60,$r2.Length))})
  }
  # 5. verify (API success != field written, must bug-view recheck, pitfall #19)
  Start-Sleep -Seconds 1
  $statusAfter="";$fpvAfter=""
  try{
    $bj2=Get-Url "$BASE/bug-view-$bid.json" 60000
    $outer2=$js.DeserializeObject($bj2)
    $inner2=$js.DeserializeObject($outer2["data"])
    $bug2=$inner2["bug"]
    $statusAfter=$bug2.Item("status")
    $fpvAfter=$bug2.Item("fixedPackageVersion")
  }catch{}
  $fpvMatch=if($fpvAfter -eq $fpv){"FPV_OK"}else{"FPV_BAD"}
  $line="BUG $bid edit=$editOk resolve=$resolveOk status=$statusAfter $fpvMatch fpv=[$fpvAfter]"
  [System.IO.File]::AppendAllText("C:\Users\user\close_bug_out.txt",$line+"`r`n",$utf8)
}
Write-Output "DONE"
