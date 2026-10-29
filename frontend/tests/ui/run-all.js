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
    console.log(`  已选产品: ${selText.trim().slice(0, 40)}`)

    // 缓存预热：**页面驱动**——直接访问目标页面并等渲染完成。此前用裸 curl
    // 预热 /api/dashboard 等端点，但页面请求带 productId+时间范围参数，业务
    // 缓存按参数分 key，预热没命中页面的 key 等于白热。页面驱动天然同款请求。
    const TEST_PID = TEST_PRODUCT || '1029'
    const warmPage = async (label, path, selector, budgetMs) => {
      const t0 = Date.now()
      await page.goto(h.BASE + path, { waitUntil: 'domcontentloaded', timeout: 60000 })
      const ok = await page.waitForSelector(selector, { timeout: budgetMs })
        .then(() => true).catch(() => false)
      console.log(`  预热 ${label}: ${ok ? 'OK' : '超时（部分数据/冷缓存）'} (${((Date.now() - t0) / 1000).toFixed(1)}s)`)
    }
    console.log('\n▶ 缓存预热（页面驱动，冷首查 60~150s 属预期）')
    await warmPage('dashboard', '/dashboard?product=' + TEST_PID, '.stat-card--bug', 150000)
    await warmPage('tasks', '/tasks?product=' + TEST_PID, '.el-table__row', 150000)
    await warmPage('timelog', '/timelog?product=' + TEST_PID, '.timelog-container', 150000)
    // 回到 dashboard 干净起点——G1 断言"打开 /?product= 应落在 dashboard"，
    // 预热留在 /timelog 会打破该假设并连锁污染后续套件
    await page.goto(h.BASE + '/dashboard?product=' + TEST_PID, { waitUntil: 'domcontentloaded', timeout: 60000 })
    await page.waitForTimeout(800)
    console.log('')

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
