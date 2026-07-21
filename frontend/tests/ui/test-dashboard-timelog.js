// Dashboard 图表 + Timelog 工时统计
const h = require('./helpers')

// 通过侧边栏导航（保持 globalSelection reactive 状态，不丢产品选择）
async function navTo(page, label) {
  await page.locator('.nav-menu .nav-item', { hasText: label }).first().click()
  await page.waitForTimeout(500)
}

async function testDashboard(page) {
  const s = h.suite('Dashboard 仪表盘')
  // 先确保产品已选（带 product query 重新进入，触发 globalSelection 恢复 + dashboard fetch）
  await h.ensureProduct(page)
  await navTo(page, '仪表盘')
  // 等 loading 结束 + 统计卡片渲染（首次 fetchData 可能较慢，用条件等待而非固定 timeout）
  await page.locator('.stats-grid .stat-card--bug').first().waitFor({ state: 'visible', timeout: 20000 }).catch(() => {})
  await page.waitForTimeout(1000) // 额外等 chart.js 渲染

  // D1 四个统计卡片
  const cards = ['.stat-card--bug', '.stat-card--story', '.stat-card--task', '.stat-card--time']
  let cardOk = true, detail = ''
  for (const sel of cards) {
    const visible = await page.locator(sel).first().isVisible().catch(() => false)
    if (!visible) { cardOk = false; detail += `${sel} 不可见; ` }
  }
  h.record(s, 'D1 四个统计卡片渲染', cardOk ? 'pass' : 'fail', detail || '全部可见')

  // 卡片数值：读取 stat-value，断言至少一个非 0（真实数据）
  const values = await page.locator('.stat-card .stat-value').allTextContents()
  const nonZero = values.filter((v) => v && v.trim() !== '0' && v.trim() !== '').length
  h.record(s, 'D1b 统计卡片有真实数值', nonZero > 0 ? 'pass' : 'warn', `非零值 ${nonZero}/${values.length}`)

  // D2 图表 canvas：按 chart-card 标题定位
  const charts = ['Bug 严重程度分布', 'Bug 类型分布', '任务状态分布']
  let chartOk = true, chartDetail = ''
  for (const title of charts) {
    const card = page.locator('.chart-card', { hasText: title }).first()
    const visible = await card.isVisible().catch(() => false)
    if (!visible) { chartOk = false; chartDetail += `${title}卡片缺失; `; continue }
    // canvas 或 chart-empty（无数据）都算正常渲染
    const hasCanvas = await card.locator('canvas').count()
    const hasEmpty = await card.locator('.chart-empty').count()
    if (hasCanvas === 0 && hasEmpty === 0) { chartOk = false; chartDetail += `${title}无canvas也无empty; ` }
  }
  h.record(s, 'D2 图表区域渲染(canvas或empty)', chartOk ? 'pass' : 'fail', chartDetail || '正常')

  // D3 查看全部链接跳转
  try {
    const link = page.locator('.list-card .list-link', { hasText: '查看全部' }).first()
    await link.click()
    await page.waitForTimeout(700)
    const url = page.url()
    h.record(s, 'D3 "查看全部"链接跳转', /\/(bugs|tasks|stories)$/.test(url) ? 'pass' : 'warn', `跳到 ${url}`)
  } catch (e) {
    h.record(s, 'D3 "查看全部"链接跳转', 'fail', e.message)
  }

  await h.shot(page, 'dashboard')
}

async function testTimelog(page) {
  const s = h.suite('Timelog 工时统计')
  await navTo(page, '工时统计')
  await h.ensureProduct(page)
  // ensureProduct 可能用 goto 重载页面，需重新导航到工时统计并等待就绪
  await page.locator('.nav-menu .nav-item', { hasText: '工时统计' }).first().click()
  await page.waitForTimeout(1000)

  // T1 快捷按钮 + 查询统计
  try {
    const quickBtn = page.locator('.quick-btn', { hasText: '本月' }).first()
    if (await quickBtn.isVisible().catch(() => false)) {
      await quickBtn.click()
      await page.waitForTimeout(300)
    }
    const queryBtn = page.locator('.el-button--primary', { hasText: '查询统计' }).first()
    await queryBtn.click()
    // 等待结果区出现（工时聚合可能较慢），最长 12 秒
    await page.locator('canvas, .empty-state').first().waitFor({ state: 'visible', timeout: 12000 }).catch(() => {})
    await page.waitForTimeout(1200) // 等 chart.js 绘制
    const toast = h.readToast ? await h.readToast(page, 1500) : ''
    h.record(s, 'T1 点"查询统计"', 'pass', toast ? `toast="${toast}"` : '无toast')
  } catch (e) {
    h.record(s, 'T1 点"查询统计"', 'fail', e.message)
  }

  // T2 结果区：查询成功后 chart.js 创建 canvas，统计卡片渲染
  const canvases = await page.locator('canvas').count().catch(() => 0)
  const statCards = await page.locator('.stat-card').count().catch(() => 0)
  h.record(s, 'T2 结果区渲染(卡片+canvas)', (canvases > 0 || statCards > 0) ? 'pass' : 'warn', `canvas=${canvases} 卡片=${statCards}`)

  await h.shot(page, 'timelog')
}

async function run(page) {
  await testDashboard(page)
  await testTimelog(page)
}

module.exports = run
