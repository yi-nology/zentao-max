#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""查看禅道 BUG 详情。

配置来自环境变量，或 ~/.zentao-credentials.env（KEY=VALUE）：
  ZT_HOST / ZT_PORT / ZT_JUMPBOX / ZT_REALM / ZT_USERNAME / ZT_PASSWORD

用法：
  python get_bug.py --bug 123456
  python get_bug.py --bug 123456 --raw
"""
import argparse
import atexit
import http.cookiejar
import json
import os
import re
import socket
import ssl
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

CRED_DEFAULT = os.path.expanduser(os.environ.get("ZT_CRED", "~/.zentao-credentials.env"))

HOST = os.environ.get("ZT_HOST", "")
JUMPBOX = os.environ.get("ZT_JUMPBOX", "")
REALM = os.environ.get("ZT_REALM", "")
PORT = int(os.environ.get("ZT_PORT") or ("18443" if JUMPBOX else "443"))

USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"
_TUNNEL_PROC = None


def load_cred():
    """读凭证文件；环境变量优先。"""
    global HOST, JUMPBOX, REALM, PORT
    vals = {}
    if os.path.exists(CRED_DEFAULT):
        with open(CRED_DEFAULT, encoding="utf-8", errors="replace") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                k, v = line.split("=", 1)
                vals[k.strip()] = v.strip().strip('"').strip("'")
    # 环境变量优先于文件
    for k in ("ZT_HOST", "ZT_PORT", "ZT_JUMPBOX", "ZT_REALM",
              "ZT_USERNAME", "ZT_PASSWORD"):
        if os.environ.get(k):
            vals[k] = os.environ[k]
    HOST = vals.get("ZT_HOST", HOST)
    if not HOST or HOST == "zentao.example.com":
        raise SystemExit("[x] 请先配置 ZT_HOST（禅道域名）")
    JUMPBOX = vals.get("ZT_JUMPBOX", JUMPBOX)
    REALM = vals.get("ZT_REALM", REALM)
    port_raw = vals.get("ZT_PORT") or os.environ.get("ZT_PORT")
    try:
        PORT = int(port_raw) if port_raw else (18443 if JUMPBOX else 443)
    except ValueError:
        raise SystemExit(f"[x] ZT_PORT 不是数字: {port_raw!r}")
    if not vals.get("ZT_USERNAME") or not vals.get("ZT_PASSWORD"):
        raise SystemExit("[x] 缺 ZT_USERNAME / ZT_PASSWORD")
    return vals


def _tunnel_args():
    return ["ssh", "-N",
            "-o", "ExitOnForwardFailure=yes",
            "-o", "ServerAliveInterval=30",
            "-o", "ServerAliveCountMax=3",
            "-o", "ConnectTimeout=15",
            "-L", f"{PORT}:{HOST}:443",
            JUMPBOX]


def _cleanup_tunnel():
    global _TUNNEL_PROC
    if _TUNNEL_PROC is not None and _TUNNEL_PROC.poll() is None:
        try:
            _TUNNEL_PROC.terminate()
        except Exception:
            pass
    _TUNNEL_PROC = None


atexit.register(_cleanup_tunnel)


def ensure_tunnel():
    global _TUNNEL_PROC
    if not JUMPBOX:
        return
    with socket.socket() as s:
        s.settimeout(1.0)
        if s.connect_ex(("127.0.0.1", PORT)) == 0:
            return
    print(f"[+] 拉起隧道: 127.0.0.1:{PORT} -> {HOST}:443 (via {JUMPBOX})", flush=True)
    _TUNNEL_PROC = subprocess.Popen(_tunnel_args(), stdout=subprocess.DEVNULL,
                                    stderr=subprocess.DEVNULL, stdin=subprocess.DEVNULL)
    for _ in range(40):
        time.sleep(0.5)
        with socket.socket() as s:
            s.settimeout(1.0)
            if s.connect_ex(("127.0.0.1", PORT)) == 0:
                return
        if _TUNNEL_PROC.poll() is not None:
            break
    _cleanup_tunnel()
    raise SystemExit("[x] 隧道 20s 内未就绪：检查跳板机 / VPN / ZT_PORT")


def patch_dns():
    if not JUMPBOX:
        return
    orig = socket.getaddrinfo
    if getattr(orig, "_zt_patched", False):
        return

    def patched(host, *a, **kw):
        return orig("127.0.0.1" if host == HOST else host, *a, **kw)

    patched._zt_patched = True
    socket.getaddrinfo = patched


class Client:
    def __init__(self, creds):
        self.creds = creds
        self.base = (f"https://{HOST}:{PORT}" if JUMPBOX
                     else (f"https://{HOST}" if PORT in (443, 0) else f"https://{HOST}:{PORT}"))
        patch_dns()
        ctx = ssl.create_default_context()
        ctx.check_hostname = False
        ctx.verify_mode = ssl.CERT_NONE
        self.jar = http.cookiejar.CookieJar()
        self.op = urllib.request.build_opener(
            urllib.request.ProxyHandler({}),
            urllib.request.HTTPSHandler(context=ctx),
            urllib.request.HTTPCookieProcessor(self.jar),
        )

    def _open(self, path, data=None, timeout=60):
        body = None
        headers = {"User-Agent": USER_AGENT}
        if data is not None:
            body = urllib.parse.urlencode(data, encoding="utf-8").encode("utf-8")
            headers["Content-Type"] = "application/x-www-form-urlencoded; charset=utf-8"
            headers["X-Requested-With"] = "XMLHttpRequest"
        req = urllib.request.Request(self.base + path, data=body, headers=headers)
        try:
            with self.op.open(req, timeout=timeout) as r:
                return r.status, r.read().decode("utf-8", "replace")
        except urllib.error.HTTPError as e:
            return e.code, e.read().decode("utf-8", "replace")
        except Exception as e:
            return 0, f"CONNERR:{e}"

    def login(self):
        st, _ = self._open("/user-login-Lw==.html")
        if st != 200:
            raise SystemExit(f"[x] 拿不到登录页 (HTTP {st})：检查隧道 / VPN / ZT_HOST")
        st, rnd = self._open("/user-refreshRandom.html")
        form = {
            "account": self.creds["ZT_USERNAME"],
            "password": self.creds["ZT_PASSWORD"],
            "verifyRand": rnd.strip(),
            "keepLogin": "0",
            "referer": "/",
        }
        if REALM:
            form["realm"] = REALM
        st, resp = self._open("/user-login.json", form)
        if '"success"' not in resp:
            raise SystemExit(f"[x] 登录失败: {resp[:300]}")
        return self

    def bug(self, bid):
        st, txt = self._open(f"/bug-view-{bid}.json")
        if st != 200:
            raise SystemExit(f"[x] 取 BUG 失败 (HTTP {st}): {txt[:300]}")
        outer = json.loads(txt)
        inner = outer.get("data")
        if isinstance(inner, str):
            inner = json.loads(inner)
        return inner


def main():
    ap = argparse.ArgumentParser(description="查看禅道 BUG 详情")
    ap.add_argument("--bug", required=True, help="BUG 编号，如 123456")
    ap.add_argument("--raw", action="store_true", help="输出完整 JSON")
    args = ap.parse_args()

    creds = load_cred()
    ensure_tunnel()
    inner = Client(creds).login().bug(args.bug)

    if args.raw:
        print(json.dumps(inner, ensure_ascii=False, indent=2))
        return

    bug = inner.get("bug", {})
    products = inner.get("products", {})
    fields = [
        ("标题", bug.get("title")),
        ("状态", bug.get("status")),
        ("类型", bug.get("type")),
        ("严重度", bug.get("severity")),
        ("优先级", bug.get("pri")),
        ("产品", products.get(str(bug.get("product")), bug.get("product"))),
        ("影响版本", bug.get("openedBuild")),
        ("修复包版本", bug.get("fixedPackageVersion")),
        ("解决版本", bug.get("resolvedBuild")),
    ]
    for k, v in fields:
        print(f"{k}: {v}")
    steps = re.sub(r"<[^>]+>", "", str(bug.get("steps") or "")).strip()
    if steps:
        print("\n重现步骤:\n" + steps)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)