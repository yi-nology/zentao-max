<template>
  <el-popover
    placement="bottom-end"
    :width="240"
    trigger="click"
    popper-class="column-settings-popper"
  >
    <template #reference>
      <el-button :icon="Setting" size="small" circle title="列设置" />
    </template>

    <div class="column-settings">
      <div class="column-settings-header">
        <span class="title">列设置</span>
        <div class="actions">
          <el-button link size="small" @click="showAll">全选</el-button>
          <el-button link size="small" @click="reset">重置</el-button>
        </div>
      </div>
      <el-divider class="divider" />
      <el-checkbox-group v-model="visibleKeys" @change="onVisibleChange">
        <div v-for="col in manageableColumns" :key="col.key" class="column-item">
          <el-checkbox :label="col.key" :value="col.key">
            {{ col.label }}
          </el-checkbox>
        </div>
      </el-checkbox-group>
    </div>
  </el-popover>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Setting } from '@element-plus/icons-vue'
import type { ColumnConfig } from '@/composables/useTableColumns'

const props = defineProps<{
  columns: ColumnConfig[]
}>()

const emit = defineEmits<{
  (e: 'toggle', key: string, visible: boolean): void
  (e: 'show-all'): void
  (e: 'reset'): void
}>()

// 用户可管理的列（排除 selection 等固定列）
const manageableColumns = computed(() =>
  props.columns.filter(c => c.key !== '__selection__' && !c.fixed)
)

// 当前可见列的 keys
const visibleKeys = computed(() =>
  manageableColumns.value.filter(c => c.visible).map(c => c.key)
)

function onVisibleChange(keys: string[]): void {
  const next = new Set(keys)
  manageableColumns.value.forEach(col => {
    emit('toggle', col.key, next.has(col.key))
  })
}

function showAll(): void {
  emit('show-all')
}

function reset(): void {
  emit('reset')
}
</script>

<style scoped>
.column-settings {
  padding: 4px 0;
}
.column-settings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 4px;
}
.column-settings-header .title {
  font-weight: 600;
  font-size: 13px;
}
.column-settings-header .actions {
  display: flex;
  gap: 4px;
}
.divider {
  margin: 8px 0;
}
.column-item {
  padding: 2px 4px;
}
.column-item :deep(.el-checkbox__label) {
  font-size: 13px;
}
</style>
