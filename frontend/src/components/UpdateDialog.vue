<script setup lang="ts">
import { onMounted, ref } from 'vue'
import api from '@/api/api'
import ChangelogDialog from '@/components/ChangelogDialog.vue'
import { renderMarkdown } from '@/utils/markdown'

// 全局新版本提示弹窗（App.vue 挂载，登录页同样生效）。
// 后端启动时已检查一次（此后 24h 周期），这里只读状态快照，不触发上游请求；
// force_update 为不可关闭模态，普通更新为可关闭弹窗。
interface UpdateStatus {
  current: string
  latest: string
  hasUpdate: boolean
  forceUpdate?: boolean
  channelUrl?: string
  downloadUrl?: string
  notes?: string
  source?: string
}

const visible = ref(false)
const force = ref(false)
const latest = ref('')
const noteHtml = ref('')
const goUrl = ref('')
const source = ref('')
const logVisible = ref(false)

onMounted(async () => {
  try {
    const res = (await api.get('/update/status')) as unknown as { data?: UpdateStatus }
    const d = res?.data
    if (!d?.hasUpdate) return
    latest.value = d.latest || ''
    force.value = !!d.forceUpdate
    noteHtml.value = renderMarkdown(d.notes || '')
    goUrl.value = d.channelUrl || d.downloadUrl || ''
    source.value = d.source || ''
    visible.value = true
  } catch {
    /* 状态获取失败不打扰用户 */
  }
})

const goUpdate = (): void => {
  if (goUrl.value) window.open(goUrl.value, '_blank', 'noopener')
  if (!force.value) visible.value = false
}
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="force ? `当前版本过旧，必须升级${latest ? `（最新版本 ${latest}）` : ''}` : `发现新版本 ${latest}`"
    width="520px"
    :show-close="!force"
    :close-on-click-modal="!force"
    :close-on-press-escape="!force"
  >
    <div class="update-note" v-html="noteHtml"></div>
    <p v-if="!noteHtml" class="update-empty">本次更新未提供说明</p>
    <template #footer>
      <el-button
        v-if="!force && source === 'release'"
        size="small"
        text
        @click="logVisible = true"
      >
        更新日志
      </el-button>
      <el-button v-if="!force" size="small" @click="visible = false">稍后再说</el-button>
      <el-button type="primary" size="small" :disabled="!goUrl" @click="goUpdate">立即更新</el-button>
    </template>
  </el-dialog>
  <ChangelogDialog v-model="logVisible" />
</template>

<style scoped>
.update-note {
  max-height: 320px;
  overflow-y: auto;
  line-height: 1.7;
  word-break: break-word;
}
.update-note :deep(h1),
.update-note :deep(h2),
.update-note :deep(h3) {
  font-size: 15px;
  margin: 10px 0 6px;
}
.update-note :deep(ul) {
  margin: 4px 0;
  padding-left: 20px;
}
.update-note :deep(p) {
  margin: 6px 0;
}
.update-note :deep(code) {
  background: var(--el-fill-color);
  border-radius: 3px;
  padding: 1px 5px;
  font-size: 12px;
}
.update-empty {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
</style>
