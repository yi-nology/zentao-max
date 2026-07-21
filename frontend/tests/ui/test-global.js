// 全局用例：路由守卫、侧边栏导航、全局产品选择器（使用共享 page）
const h = require('./helpers')

async function run(page) {
  const s = h.suite('全局/外壳')
  const errors = page._errors || []
  try {
    // G1 路由守卫：已初始化态访问 / 应进入 /dashboard（而非 init-guide）
    // URL 可能带 query（如 ?product=200），只断言 path 部分
    const url = page.url()
    const path = new URL(url).pathname
    h.record(s, 'G1 路由守卫进入 /dashboard', path === '/dashboard' ? 'pass' : 'fail', `path=${path}`)

    // G2 侧边栏导航：逐个点击并断言 path 与 active 态
    const navMap = [
      { label: 'Bug 查询', path: '/bugs' },
      { label: '需求查询', path: '/stories' },
      { label: '任务查询', path: '/tasks' },
      { label: '工时统计', path: '/timelog' },
      { label: '定时任务', path: '/scheduler' },
      { label: '心跳检测', path: '/health' },
      { label: 'MCP 对接', path: '/mcp-guide' },
      { label: '设置', path: '/settings' },
      { label: '仪表盘', path: '/dashboard' },
    ]
    let navFail = ''
    for (const item of navMap) {
      await page.keyboard.press('Escape').catch(() => {}) // 清理可能残留的弹窗
      const link = page.locator('.nav-menu .nav-item', { hasText: item.label }).first()
      await link.click()
      // 等 URL path 变化 + 适当间隔（避免上一次导航过渡期点击落空）
      await page.waitForFunction(
        (exp) => location.pathname === exp,
        item.path,
        { timeout: 6000 },
      ).catch(() => {})
      // 心跳检测页会发起较慢的 /api/healthz，进入后多等一会，避免下一次点击落在导航过渡期
      const waitMs = item.label === '心跳检测' ? 1500 : 700
      await page.waitForTimeout(waitMs)
      const curPath = new URL(page.url()).pathname
      const isActive = await link.evaluate((el) => el.classList.contains('active')).catch(() => false)
      if (curPath !== item.path || !isActive) {
        navFail += `${item.label}→${item.path}(实际=${curPath},active=${isActive}); `
      }
    }
    // 导航测完回到 dashboard，让后续套件有干净起点
    await page.locator('.nav-menu .nav-item', { hasText: '仪表盘' }).first().click()
    await page.waitForTimeout(500)
    h.record(s, 'G2 侧边栏导航 9 项跳转+active态', navFail ? 'fail' : 'pass', navFail || '全部跳转正常且高亮')

    // G3 产品选择器已选产品（run-all 已通过 query 设置）
    const selText = await page.locator('.product-selector .el-select__wrapper').first().textContent().catch(() => '')
    const productSelected = selText.trim().length > 0 && !selText.includes('请选择产品')
    h.record(s, 'G3 全局产品已选', productSelected ? 'pass' : 'fail', `显示: ${selText.trim().slice(0, 30)}`)

    // G4 账户状态点
    const dotVisible = await page.locator('.account-status-dot').first().isVisible().catch(() => false)
    h.record(s, 'G4 账户状态点(.account-status-dot)渲染', dotVisible ? 'pass' : 'warn', dotVisible ? '可见' : '不可见')

    await h.shot(page, 'global-layout')

    if (errors.length) {
      h.recordIssue('全局页面', `捕获 ${errors.length} 条 console/page 错误: ${errors.slice(0, 3).map((e) => e.msg).join(' | ')}`)
    } else {
      h.record(s, 'G5 无运行期 JS 错误', 'pass')
    }
  } catch (e) {
    h.record(s, '全局套件执行', 'fail', e.message)
  }
}

module.exports = run
