// UI 测试主入口：共享一个 browser 会话，只 openApp 一次（避免反复触发路由守卫/限流）
const h = require('./helpers')

const suites = [
  ['./test-global', '全局/外壳'],
  ['./test-dashboard-timelog', 'Dashboard/Timelog'],
  ['./test-lists', '业务列表页'],
  ['./test-misc', 'Scheduler/Health/MCP/Settings/Init'],
]

// 用一个有真实数据的产品打开首页（product=200：bugs=3 stories=8 tasks=172），
// 让 Layout 从 URL query 恢复 globalSelection，后续侧边栏导航共享该 reactive 状态。
const TEST_PRODUCT = process.env.TEST_PRODUCT || '200'

;(async () => {
  console.log('zentao-max UI 测试开始')
  console.log(`目标: ${h.BASE} | headless chromium | 共享会话 | 测试产品ID: ${TEST_PRODUCT}`)
  console.log(`截图目录: ${h.SHOT_DIR}\n`)

  const { browser, page } = await h.sharedSession()
  try {
    console.log('▶ 初始化：打开应用（带测试产品 query）')
    await page.goto(h.BASE + '/?product=' + TEST_PRODUCT, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await page.waitForSelector('.nav-menu', { timeout: 20000 })
    await page.waitForTimeout(1200)
    const selText = await page.locator('.product-selector .el-select__wrapper').first().textContent().catch(() => '?')
    console.log(`  已选产品: ${selText.trim().slice(0, 40)}\n`)

    for (const [file, label] of suites) {
      console.log(`\n▶ 运行套件: ${label} (${file})`)
      console.log('-'.repeat(60))
      try {
        const run = require(file)
        await run(page)
      } catch (e) {
        console.error(`  ✗ 套件异常: ${e.message}`)
        console.error(e.stack)
      }
    }
  } finally {
    await browser.close()
  }

  const code = h.summarize()
  process.exit(code)
})()
