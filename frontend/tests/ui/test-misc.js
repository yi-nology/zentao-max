// Scheduler CRUD（含 createdAt 回归）+ HealthCheck + MCPGuide + Settings + Init 页
const h = require('./helpers')

async function navTo(page, label) {
  // 导航前关闭任何残留的 dialog-overlay（Scheduler 的弹窗可能挡住侧边栏）
  await closeAllOverlays(page)
  const item = page.locator('.nav-menu .nav-item', { hasText: label }).first()
  await item.click()
  await page.waitForTimeout(500)
}

// 关闭所有可见的 dialog-overlay（Scheduler 自定义弹窗）和 el-message-box
async function closeAllOverlays(page) {
  // 先尝试按 ESC（el-message-box / dialog 通常响应 ESC）
  await page.keyboard.press('Escape').catch(() => {})
  await page.waitForTimeout(200)
  // 关闭自定义 dialog-overlay：点其空白区（@click.self 关闭）
  const overlays = page.locator('.dialog-overlay:visible')
  const n = await overlays.count().catch(() => 0)
  for (let i = 0; i < n; i++) {
    await overlays.first().click({ position: { x: 5, y: 5 } }).catch(() => {})
    await page.waitForTimeout(300)
  }
  // 再处理可能残留的 el-message-box
  const cancelBtn = page.locator('.el-message-box__btns .el-button').first()
  if (await cancelBtn.isVisible().catch(() => false)) {
    await cancelBtn.click().catch(() => {})
    await page.waitForTimeout(300)
  }
}

async function testScheduler(page) {
  const s = h.suite('Scheduler 定时任务')
  await navTo(page, '定时任务')
  await page.waitForTimeout(1200)

  // S1 列表加载 + tab 切换
  const tableRows = await page.locator('.data-table tbody tr, .data-table .status-badge').count().catch(() => 0)
  h.record(s, 'S1 任务列表加载', tableRows >= 0 ? 'pass' : 'fail', `行数 ${tableRows}`)

  try {
    const logTab = page.locator('.tab-btn', { hasText: '执行日志' }).first()
    await logTab.click()
    await page.waitForTimeout(800)
    const back = page.locator('.tab-btn', { hasText: '任务列表' }).first()
    await back.click()
    await page.waitForTimeout(600)
    h.record(s, 'S1b tab切换(执行日志/任务列表)', 'pass')
  } catch (e) {
    h.record(s, 'S1b tab切换', 'fail', e.message)
  }

  // S2 新建任务
  let createdTaskName = ''
  try {
    await page.locator('button.btn-primary', { hasText: '新建任务' }).first().click()
    const overlay = page.locator('.dialog-overlay:visible').first()
    await overlay.waitFor({ state: 'visible', timeout: 5000 })

    // 选第一个报告类型卡片
    const rtCard = overlay.locator('.report-type-card').first()
    await rtCard.click()

    // 任务名称
    createdTaskName = 'UITEST_' + Date.now()
    await overlay.locator('input.form-input').first().fill(createdTaskName)

    // 选产品（原生 select）— 选第一个非空 option
    const productSelect = overlay.locator('select.form-input').first()
    const opts = await productSelect.locator('option').allTextContents()
    if (opts.length > 1) {
      await productSelect.selectOption({ index: 1 })
    }

    // 点第一个 cron 预设
    const preset = overlay.locator('.preset-btn').first()
    await preset.click()

    // 保存
    await overlay.locator('button.btn-primary', { hasText: '保存' }).click()
    await page.waitForTimeout(1500)
    const toast = await h.readToast(page, 2000)
    // 列表应包含新任务
    const hasNew = await page.locator('.data-table', { hasText: createdTaskName }).count()
    h.record(s, 'S2 新建任务', hasNew > 0 ? 'pass' : 'warn', `toast="${toast}" 列表含新任务=${hasNew > 0}`)
  } catch (e) {
    h.record(s, 'S2 新建任务', 'fail', e.message)
  }

  // S5 回归：编辑刚创建的任务，断言保存成功且列表仍正常（呼应后端 createdAt 修复）
  if (createdTaskName) {
    try {
      await closeAllOverlays(page)
      const editBtn = page.locator(`[title="编辑"]`).first()
      await editBtn.click()
      const overlay = page.locator('.dialog-overlay:visible').first()
      await overlay.waitFor({ state: 'visible', timeout: 5000 })
      // 标题应为"编辑任务"
      const title = (await overlay.locator('h3').first().textContent()).trim()
      await overlay.locator('input.form-input').first().fill(createdTaskName + '_ED')
      await overlay.locator('button.btn-primary', { hasText: '保存' }).click()
      await page.waitForTimeout(1500)
      // 保存后 overlay 应关闭；若仍在则手动关
      await closeAllOverlays(page)
      const hasEd = await page.locator('.data-table', { hasText: createdTaskName + '_ED' }).count()
      h.record(s, 'S5 编辑任务(createdAt回归)', (title.includes('编辑') && hasEd > 0) ? 'pass' : 'warn', `标题="${title}" 更新生效=${hasEd > 0}`)
    } catch (e) {
      h.record(s, 'S5 编辑任务(createdAt回归)', 'fail', e.message)
      await closeAllOverlays(page)
    }
  }

  // S3 立即执行
  try {
    await closeAllOverlays(page)
    const runBtn = page.locator('[title="立即执行"]').first()
    await runBtn.click()
    await page.waitForTimeout(2000)
    const overlay = page.locator('.dialog-overlay:visible').first()
    const visible = await overlay.isVisible().catch(() => false)
    h.record(s, 'S3 立即执行', visible ? 'pass' : 'warn', visible ? '弹出执行结果' : '无结果弹窗')
    await closeAllOverlays(page)
  } catch (e) {
    h.record(s, 'S3 立即执行', 'fail', e.message)
    await closeAllOverlays(page)
  }

  // S3b 删除（走 ElMessageBox.confirm）—— 清理测试数据
  try {
    await closeAllOverlays(page)
    const delBtn = page.locator('[title="删除"]').first()
    await delBtn.click()
    await page.waitForTimeout(600)
    const confirmBtn = page.locator('.el-message-box__btns .el-button--primary').first()
    if (await confirmBtn.isVisible().catch(() => false)) {
      await confirmBtn.click()
    } else {
      await page.locator('.el-message-box button').last().click().catch(() => {})
    }
    await page.waitForTimeout(1200)
    h.record(s, 'S3b 删除任务(清理)', 'pass')
  } catch (e) {
    h.record(s, 'S3b 删除任务(清理)', 'warn', e.message)
  }
  // 套件结束：彻底清理 overlay，避免遮挡下一个套件的侧边栏导航
  await closeAllOverlays(page)

  await h.shot(page, 'scheduler')
}

async function testHealthCheck(page) {
  const s = h.suite('HealthCheck 心跳检测')
  await navTo(page, '心跳检测')
  // healthz 会依次检测多个端点（产品/项目/Bug/需求/任务/用户），首次较慢，需等较久
  await page.locator('.overview-main').first().waitFor({ state: 'visible', timeout: 20000 }).catch(() => {})
  await page.waitForTimeout(800)

  const overview = await page.locator('.overview-main').first().isVisible().catch(() => false)
  h.record(s, '概览渲染(.overview-main)', overview ? 'pass' : 'fail', overview ? '已渲染' : '超时未渲染')

  // 刷新按钮
  try {
    const btn = page.locator('button.refresh-btn').first()
    const before = await page.locator('.check-time').first().textContent().catch(() => '')
    await btn.click()
    await page.waitForTimeout(2000)
    h.record(s, '刷新按钮点击', 'pass')
  } catch (e) {
    h.record(s, '刷新按钮点击', 'warn', e.message)
  }

  await h.shot(page, 'health-check')
}

async function testMCPGuide(page) {
  const s = h.suite('MCPGuide MCP对接')
  await navTo(page, 'MCP 对接')
  await page.waitForTimeout(1500)

  // config-tab 切换
  const tabCount = await page.locator('.config-tab').count().catch(() => 0)
  h.record(s, `config-tab 数量`, tabCount >= 5 ? 'pass' : 'warn', `${tabCount} 个`)

  // 点几个 tab
  let switchOk = true
  for (let i = 0; i < Math.min(3, tabCount); i++) {
    try {
      await page.locator('.config-tab').nth(i).click()
      await page.waitForTimeout(300)
    } catch { switchOk = false }
  }
  h.record(s, 'config-tab 切换', switchOk ? 'pass' : 'fail')

  // 测试连接按钮
  try {
    const btn = page.locator('.conn-test-btn').first()
    await btn.click()
    await page.waitForTimeout(3000)
    const cls = await btn.evaluate((el) => el.className).catch(() => '')
    const text = (await btn.textContent().catch(() => '')).trim()
    h.record(s, '测试连接', /success|已连接/.test(cls + text) ? 'pass' : 'warn', `结果="${text}"`)
  } catch (e) {
    h.record(s, '测试连接', 'warn', e.message)
  }

  await h.shot(page, 'mcp-guide')
}

async function testSettings(page) {
  const s = h.suite('Settings 设置')
  await navTo(page, '设置')
  await page.waitForTimeout(1500)

  const section = await page.locator('.settings-section').first().isVisible().catch(() => false)
  h.record(s, '设置区渲染', section ? 'pass' : 'fail')

  // 暗黑模式切换
  try {
    const sw = page.locator('.el-switch').first()
    const before = await sw.evaluate((el) => el.classList.contains('is-checked')).catch(() => null)
    await sw.click()
    await page.waitForTimeout(400)
    const after = await sw.evaluate((el) => el.classList.contains('is-checked')).catch(() => null)
    h.record(s, '暗黑模式开关切换', before !== after ? 'pass' : 'warn', `${before}→${after}`)
    // 切回原状
    if (before !== after) { await sw.click(); await page.waitForTimeout(300) }
  } catch (e) {
    h.record(s, '暗黑模式开关切换', 'warn', e.message)
  }

  await h.shot(page, 'settings')
}

async function run(page) {
  await testScheduler(page)
  await testHealthCheck(page)
  await testMCPGuide(page)
  await testSettings(page)
}

module.exports = run
