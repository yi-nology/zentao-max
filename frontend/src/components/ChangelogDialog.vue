<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import api from '@/api/api'
import { renderMarkdown } from '@/utils/markdown'

// 历史更新日志（What's New）。设置页与更新弹窗共用；仅在检查源为
// 版本发布管理服务（source=release）时展示入口。
interface LogItem {
  version: string
  releasedAt: string
  noteMd: string
}

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v)
})

const loading = ref(false)
const items = ref<LogItem[]>([])
const loadError = ref('')
const loaded = ref(false)

watch(visible, (open) => {
  if (open && !loaded.value) load()
})

const load = async (): Promise<void> => {
  loading.value = true
  loadError.value = ''
  try {
    const res = (await api.get('/update/changelog', { params: { limit: 20 } })) as unknown as {
      data?: LogItem[]
    }
    items.value = res?.data || []
    loaded.value = true
  } catch (e) {
    const resp = (e as { response?: { data?: { message?: string } } })?.response
    loadError.value = resp?.data?.message || (e instanceof Error ? e.message : '加载失败')
  } finally {
    loading.value = false
  }
}

const formatDate = (iso: string): string => (iso ? iso.slice(0, 10) : '')
</script>

<template>
  <el-dialog v-model="visible" title="更新日志" width="560px" append-to-body>
    <div v-loading="loading" class="changelog">
      <el-alert v-if="loadError && !loading" :title="loadError" type="error" :closable="false" />
      <el-empty v-else-if="!loading && !items.length" description="暂无更新日志" />
      <template v-else>
        <div v-for="it in items" :key="it.version" class="log-item">
          <div class="log-head">
            <span class="log-version">v{{ it.version }}</span>
            <span class="log-date">{{ formatDate(it.releasedAt) }}</span>
          </div>
          <div v-if="it.noteMd" class="log-note" v-html="renderMarkdown(it.noteMd)"></div>
          <p v-else class="log-empty">本次更新未提供说明</p>
        </div>
      </template>
    </div>
  </el-dialog>
</template>

<style scoped>
.changelog {
  min-height: 120px;
  max-height: 420px;
  overflow-y: auto;
}
.log-item {
  padding: 10px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.log-item:last-child {
  border-bottom: none;
}
.log-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 4px;
}
.log-version {
  font-weight: 600;
  color: var(--el-color-primary);
}
.log-date {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.log-note {
  line-height: 1.7;
  word-break: break-word;
  font-size: 13px;
}
.log-note :deep(h1),
.log-note :deep(h2),
.log-note :deep(h3) {
  font-size: 14px;
  font-weight: 600;
  margin: 6px 0 4px;
}
.log-note :deep(ul) {
  margin: 4px 0;
  padding-left: 18px;
}
.log-note :deep(p) {
  margin: 4px 0;
}
.log-note :deep(code) {
  background: var(--el-fill-color);
  border-radius: 3px;
  padding: 1px 5px;
  font-size: 12px;
}
.log-empty {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
</style>
