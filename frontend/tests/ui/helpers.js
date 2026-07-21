// 公共辅助：浏览器启动、错误捕获、Element Plus 交互、截图、报告收集
const { chromium } = require('playwright')
const fs = require('fs')
const path = require('path')

const BASE = process.env.BASE_URL || 'http://localhost:6100'
const SHOT_DIR = path.join(__dirname, 'screenshots')
if (!fs.existsSync(SHOT_DIR)) fs.mkdirSync(SHOT_DIR, { recursive: true })

// 预期可忽略的控制台/网络噪音（降级 toast、无禅道时的网络失败等不算 bug）
const IGNORE_PATTERNS = [
  /favicon/i,
  /Prefer to debug/,
]

// 全局结果收集：{ suite, cases: [{name, status, detail}], issues: [] }
const report = { suites: [], issues: [] }

function suite(name) {
  const s = { name, cases: [] }
  report.suites.push(s)
  return s
}

// 记录一条用例结果。status: 'pass' | 'fail' | 'warn'
function record(s, name, status, detail = '') {
  s.cases.push({ name, status, detail })
  const icon = status === 'pass' ? '✓' : status === 'warn' ? '!' : '✗'
  console.log(`  ${icon} [${status.toUpperCase()}] ${name}${detail ? ' — ' + detail : ''}`)
}

function recordIssue(where, desc) {
  report.issues.push({ where, desc })
  console.log(`  ⚠ ISSUE @ ${where}: ${desc}`)
}

// 共享会话：全测试只开一个 browser/context/page，只 openApp 一次，
// 避免反复触发路由守卫的 getInitStatus（后端有限流）。
// 同时通过 route 节流，把 /api 请求速率限制在限流阈值之下（旧后端默认 60/分钟）。
let _shared = null
async function sharedSession() {
  if (_shared) return _shared
  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()

  const errors = []
  page.on('pageerror', (err) => {
    const msg = String(err.message || err)
    if (!IGNORE_PATTERNS.some((re) => re.test(msg))) errors.push({ type: 'pageerror', msg })
  })
  page.on('console', (msg) => {
    if (msg.type() !== 'error') return
    const text = msg.text()
    if (IGNORE_PATTERNS.some((re) => re.test(text))) return
    errors.push({ type: 'console', msg: text })
  })

  // 修复版后端限流已放宽到 600/分钟 + 健康/状态端点免限流，
  // 全量 UI 测试请求数远低于阈值，无需节流。保持请求自然速度，避免拖慢测试时序。

  _shared = { browser, context, page, errors }
  return _shared
}

// 启动浏览器 + 一个 page，并挂载错误捕获（独立会话，仅在需要隔离时用）
async function newPage() {
  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()

  const errors = []
  page.on('pageerror', (err) => {
    const msg = String(err.message || err)
    if (!IGNORE_PATTERNS.some((re) => re.test(msg))) errors.push({ type: 'pageerror', msg })
  })
  page.on('console', (msg) => {
    if (msg.type() !== 'error') return
    const text = msg.text()
    if (IGNORE_PATTERNS.some((re) => re.test(text))) return
    errors.push({ type: 'console', msg: text })
  })

  return { browser, context, page, errors }
}

// 确保已选产品（globalSelection.product 非空）。
// 若当前未选，通过 URL query 设置默认测试产品并重新进入当前页。
async function ensureProduct(page, productId = '200') {
  const selText = await page.locator('.product-selector .el-select__wrapper').first().textContent().catch(() => '')
  if (selText && !selText.includes('请选择产品')) return true
  // 未选，用 URL query 恢复 globalSelection
  const cur = new URL(page.url())
  cur.searchParams.set('product', productId)
  await page.goto(cur.toString(), { waitUntil: 'domcontentloaded' }).catch(() => {})
  await page.waitForTimeout(1000)
  return false
}

// 不真正关闭，仅用于兼容旧的 try/finally（共享会话需保留到最后）
async function keepAlive() {}

// 打开应用首页并等待路由守卫 getInitStatus 完成、Layout 渲染
async function openApp(page) {
  await page.goto(BASE + '/', { waitUntil: 'domcontentloaded', timeout: 30000 })
  // 路由守卫会调 /api/init/status，等侧边栏出现即表示已进入主界面
  await page.waitForSelector('.nav-menu', { timeout: 20000 })
  return page
}

// Element Plus el-select 选择某个选项（按 placeholder 定位触发器，再点下拉项文本）
// 注意：必须点击内部的 .el-select__wrapper 才能打开下拉（点外层 .el-select 无效）
// scopeSelector: 限定 el-select 所在的祖先容器选择器（默认全局）
async function elSelect(page, placeholder, optionText, scopeSelector = '') {
  const scope = scopeSelector ? page.locator(scopeSelector) : page
  const sel = scope.locator(`.el-select`, { hasText: placeholder }).first()
  await sel.scrollIntoViewIfNeeded().catch(() => {})
  await sel.locator('.el-select__wrapper').first().click()
  // el-select 下拉渲染到 body 的 .el-select__popper
  const dropdown = page.locator('.el-select__popper:visible .el-select-dropdown__item', { hasText: optionText }).first()
  await dropdown.waitFor({ state: 'visible', timeout: 5000 })
  await dropdown.click()
}

// 选产品（全局 ProductSelector，在 header 内）。返回所选产品名。
// 必须点击 .el-select__wrapper 才能打开下拉。
async function selectProduct(page, index = 0) {
  const sel = page.locator('.product-selector .el-select').first()
  await sel.waitFor({ state: 'visible', timeout: 10000 })
  await sel.locator('.el-select__wrapper').first().click()
  const items = page.locator('.el-select__popper:visible .el-select-dropdown__item')
  await items.first().waitFor({ state: 'visible', timeout: 8000 })
  const count = await items.count()
  if (count === 0) throw new Error('产品列表为空，无法选择产品')
  const target = items.nth(Math.min(index, count - 1))
  const name = (await target.textContent()).trim()
  await target.click()
  // 等下拉关闭 + globalSelection 写入 URL query
  await page.waitForTimeout(700)
  return name
}

// 截图
async function shot(page, name) {
  const file = path.join(SHOT_DIR, `${name}.png`)
  await page.screenshot({ path: file, fullPage: true }).catch(() => {})
  return file
}

// 等待并断言 el-table 有数据行（返回行数）
async function waitTableRows(page, tableSelector = '.el-table') {
  await page.waitForSelector(`${tableSelector} .el-table__row`, { timeout: 15000 })
  return page.locator(`${tableSelector} .el-table__row`).count()
}

// 等待 v-loading 消失
async function waitLoadingDone(page) {
  await page.waitForSelector('.el-loading-mask', { state: 'detached', timeout: 15000 }).catch(() => {})
}

// 等待并读取 el-message（toast）文本；match 传入则要求包含该子串
async function readToast(page, timeout = 4000) {
  try {
    const toast = page.locator('.el-message').first()
    await toast.waitFor({ state: 'visible', timeout })
    const text = (await toast.textContent()).trim()
    return text
  } catch {
    return ''
  }
}

// 打印最终汇总报告并返回退出码
function summarize() {
  console.log('\n' + '='.repeat(70))
  console.log('UI 测试汇总')
  console.log('='.repeat(70))
  let pass = 0, fail = 0, warn = 0
  for (const s of report.suites) {
    console.log(`\n【${s.name}】`)
    for (const c of s.cases) {
      const icon = c.status === 'pass' ? '✓' : c.status === 'warn' ? '!' : '✗'
      console.log(`  ${icon} ${c.name}${c.detail ? ' — ' + c.detail : ''}`)
      if (c.status === 'pass') pass++
      else if (c.status === 'warn') warn++
      else fail++
    }
  }
  console.log('\n' + '-'.repeat(70))
  console.log(`用例: ${report.suites.reduce((n, s) => n + s.cases.length, 0)} | 通过 ${pass} | 警告 ${warn} | 失败 ${fail}`)
  if (report.issues.length) {
    console.log(`\n发现 ${report.issues.length} 个待处理问题:`)
    report.issues.forEach((i, idx) => console.log(`  ${idx + 1}. [${i.where}] ${i.desc}`))
  }
  console.log(`\n截图目录: ${SHOT_DIR}`)
  console.log('='.repeat(70))
  return fail > 0 ? 1 : 0
}

module.exports = {
  BASE, SHOT_DIR, report,
  suite, record, recordIssue,
  newPage, sharedSession, keepAlive, ensureProduct,
  openApp,
  elSelect, selectProduct,
  shot, waitTableRows, waitLoadingDone, readToast,
  summarize,
}
