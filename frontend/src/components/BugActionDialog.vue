<template>
    <!-- Bug 编辑对话框（bug-edit 全量更新：质量五字段一次写全 + 原值透传） -->
    <el-dialog v-model="editDialogVisible" title="编辑 Bug / 质量分析字段" width="640px" append-to-body>
      <el-form label-width="100px">
        <el-form-item label="修复包版本">
          <el-input v-model="editForm.fixedPackageVersion" placeholder="包名=版本号（等号连接，如 example-pkg=1.0.0-1）" />
        </el-form-item>
        <el-form-item label="原因分析">
          <el-input v-model="editForm.reasonAnalysis" type="textarea" :rows="3" placeholder="什么组件、什么功能、什么情况触发、造成什么后果" />
        </el-form-item>
        <el-form-item label="处理意见">
          <el-input v-model="editForm.advice" type="textarea" :rows="3" placeholder="采用什么补丁、修复思路、改动范围、是否影响接口/ABI" />
        </el-form-item>
        <el-form-item label="实现说明">
          <el-input v-model="editForm.implementation" type="textarea" :rows="3" placeholder="应用哪个 patch、修改哪个文件、主要改动点" />
        </el-form-item>
        <el-form-item label="测试建议">
          <el-input v-model="editForm.testAdvise" type="textarea" :rows="3" placeholder="怎么构造测试用例、预期结果、影响域" />
        </el-form-item>
        <el-form-item label="解决版本">
          <el-input v-model="editForm.resolvedBuild" placeholder="可选，如 mainV3_Build06" />
        </el-form-item>
        <el-form-item label="追加备注">
          <el-input v-model="editForm.comment" type="textarea" :rows="2" placeholder="可选，追加到 Bug 备注" />
        </el-form-item>
      </el-form>
      <div class="edit-warn">
        禅道 bug-edit 为全量更新：提交时将自动带上标题/版本/执行的原值；接口返回 success 不代表字段落库，提交后会回读刷新列表，请核对详情。
      </div>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="editSubmitting" @click="submitBugEdit">提交</el-button>
      </template>
    </el-dialog>

    <!-- Bug 记录工时（走批量工时端点，objectType=bug_xxx） -->
    <el-dialog v-model="effortDialogVisible" title="记录工时" width="480px" append-to-body>
      <el-form label-width="90px">
        <el-form-item label="日期">
          <el-date-picker v-model="effortForm.date" type="date" value-format="YYYY-MM-DD" :disabled-date="(d: Date) => d.getTime() > Date.now()" style="width: 100%" />
        </el-form-item>
        <el-form-item label="工作内容">
          <el-input v-model="effortForm.work" type="textarea" :rows="2" placeholder="做了什么（将写入禅道日志）" />
        </el-form-item>
        <el-form-item label="消耗(h)">
          <el-input-number v-model="effortForm.consumed" :min="0.5" :max="24" :step="0.5" :precision="1" style="width: 100%" />
        </el-form-item>
      </el-form>
      <div class="edit-warn">
        工时提交后可在「工时日历」回查；部分实例的工时同步外部系统后不可改删，请确认内容后再提交。
      </div>
      <template #footer>
        <el-button @click="effortDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="effortSubmitting" @click="submitBugEffort">提交</el-button>
      </template>
    </el-dialog>

    <!-- 简单动作对话框（确认/解决/关闭/激活/指派 通用） -->
    <el-dialog v-model="actionDialogVisible" :title="actionTitle" width="480px" append-to-body>
      <el-form label-width="90px">
        <el-form-item v-if="actionType === 'resolve'" label="解决方案">
          <el-select v-model="actionForm.resolution" style="width: 100%">
            <el-option label="已解决(bydesign)" value="bydesign" />
            <el-option label="重复(duplicate)" value="duplicate" />
            <el-option label="外部原因(external)" value="external" />
            <el-option label="修复(fixed)" value="fixed" />
            <el-option label="不予解决(notrepro)" value="notrepro" />
            <el-option label="延期(postponed)" value="postponed" />
            <el-option label="不予修复(willnotfix)" value="willnotfix" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="actionType === 'resolve'" label="解决版本">
          <el-input v-model="actionForm.resolvedBuild" placeholder="如 mainV3_Build06" />
        </el-form-item>
        <el-form-item v-if="actionType === 'assign'" label="指派给">
          <el-input v-model="actionForm.assignedTo" placeholder="账号" />
        </el-form-item>
        <el-form-item v-if="actionType === 'activate'" label="指派给">
          <el-input v-model="actionForm.assignedTo" placeholder="账号（可空）" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="actionForm.comment" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="actionDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="actionLoading" @click="submitAction">提交</el-button>
      </template>
    </el-dialog>
</template>

<script setup lang="ts">
/**
 * Bug 写操作弹窗（issue #7-5：从 Bugs.vue 拆出）：确认/解决/关闭/激活/指派
 * 通用动作 + 质量分析字段编辑（bug-edit 全量更新，必填字段带原值透传）+
 * 记录工时（批量工时端点 objectType=bug_xxx）。
 * 打开方式：dialogRef.openAction(type, id) / openEdit(bug) / openEffort(bug)；
 * 写成功经 done 事件通知父组件回读刷新列表。
 */
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { resolveBug, closeBug, assignBug, confirmBug, activateBug, updateBug, batchCreateEffort } from '@/api/zentao'
import { extractApiError } from '@/api/api'
import { formatDate as toLocalDateStr } from '@/utils/date'
import type { Bug } from '@/types/api'

const emit = defineEmits<{ done: [] }>()

// ----- 简单动作（确认/解决/关闭/激活/指派 通用）-----
type BugActionType = 'confirm' | 'resolve' | 'close' | 'activate' | 'assign'
const actionDialogVisible = ref(false)
const actionLoading = ref(false)
const actionType = ref<BugActionType>('confirm')
const actionBugId = ref(0)
const actionForm = reactive<{ resolution: string; resolvedBuild: string; assignedTo: string; comment: string }>({
  resolution: 'fixed',
  resolvedBuild: '',
  assignedTo: '',
  comment: ''
})
const actionTitle = computed(() => {
  const map: Record<BugActionType, string> = {
    confirm: '确认 Bug', resolve: '解决 Bug', close: '关闭 Bug', activate: '激活 Bug', assign: '指派 Bug'
  }
  return map[actionType.value]
})

const handleBugAction = (type: BugActionType, id: number): void => {
  actionType.value = type
  actionBugId.value = id
  actionForm.resolution = 'fixed'
  actionForm.resolvedBuild = ''
  actionForm.assignedTo = ''
  actionForm.comment = ''
  actionDialogVisible.value = true
}

const submitAction = async (): Promise<void> => {
  actionLoading.value = true
  try {
    const id = actionBugId.value
    if (actionType.value === 'confirm') {
      await confirmBug(id, { comment: actionForm.comment })
    } else if (actionType.value === 'resolve') {
      await resolveBug(id, { resolution: actionForm.resolution, resolvedBuild: actionForm.resolvedBuild, comment: actionForm.comment })
    } else if (actionType.value === 'close') {
      await closeBug(id, { comment: actionForm.comment })
    } else if (actionType.value === 'activate') {
      await activateBug(id, { assignedTo: actionForm.assignedTo, comment: actionForm.comment })
    } else if (actionType.value === 'assign') {
      await assignBug(id, { assignedTo: actionForm.assignedTo, comment: actionForm.comment })
    }
    ElMessage.success('操作成功')
    actionDialogVisible.value = false
    emit('done')
  } catch (err) {
    ElMessage.error(extractApiError(err) || '操作失败')
  } finally {
    actionLoading.value = false
  }
}

// ----- 编辑（质量分析五字段等，bug-edit 全量更新）-----
const editDialogVisible = ref(false)
const editSubmitting = ref(false)
const editBugId = ref(0)
const editBug = ref<Bug | null>(null)
const editForm = reactive<{
  fixedPackageVersion: string
  reasonAnalysis: string
  advice: string
  implementation: string
  testAdvise: string
  resolvedBuild: string
  comment: string
}>({
  fixedPackageVersion: '',
  reasonAnalysis: '',
  advice: '',
  implementation: '',
  testAdvise: '',
  resolvedBuild: '',
  comment: ''
})

// ----- 记录工时（批量工时端点，objectType=bug_xxx）-----
const effortDialogVisible = ref(false)
const effortSubmitting = ref(false)
const effortBugId = ref(0)
const effortExecution = ref(0)
const effortForm = reactive<{ date: string; work: string; consumed: number }>({
  date: toLocalDateStr(new Date()),
  work: '',
  consumed: 0.5
})

const openBugEffortDialog = (bug: Bug): boolean => {
  if (!bug.execution) {
    ElMessage.warning('该 Bug 未关联执行，无法记录工时（受限实例要求工时必须挂执行）')
    return false
  }
  effortBugId.value = bug.id
  effortExecution.value = bug.execution || 0
  effortForm.date = toLocalDateStr(new Date())
  // 工作内容预填 Bug 标题，日报可读
  effortForm.work = String(bug.title || '')
  effortForm.consumed = 0.5
  effortDialogVisible.value = true
  return true
}

const submitBugEffort = async (): Promise<void> => {
  if (!effortForm.date || !effortForm.work.trim() || effortForm.consumed <= 0) {
    ElMessage.error('日期、工作内容、消耗（>0）都不能为空')
    return
  }
  effortSubmitting.value = true
  try {
    const res = await batchCreateEffort({
      date: effortForm.date,
      items: [{
        work: effortForm.work.trim(),
        objectType: `bug_${effortBugId.value}`,
        execution: effortExecution.value,
        consumed: effortForm.consumed,
        left: 0
      }]
    })
    if (res.code === 200 && res.data && res.data.created > 0) {
      ElMessage.success(`工时已记录（${effortForm.date} ${effortForm.consumed}h），可在工时日历回查`)
      effortDialogVisible.value = false
    } else {
      ElMessage.error(res.message || '提交失败')
    }
  } catch (err) {
    ElMessage.error(extractApiError(err) || '提交失败')
  } finally {
    effortSubmitting.value = false
  }
}

const openBugEditDialog = (bug: Bug): void => {
  editBugId.value = bug.id
  editBug.value = bug
  editForm.fixedPackageVersion = ''
  editForm.reasonAnalysis = ''
  editForm.advice = ''
  editForm.implementation = ''
  editForm.testAdvise = ''
  editForm.resolvedBuild = ''
  editForm.comment = ''
  editDialogVisible.value = true
}

const submitBugEdit = async (): Promise<void> => {
  const bug = editBug.value
  if (!bug) return
  editSubmitting.value = true
  try {
    // bug-edit 是全量更新：必填的 title/openedBuild/execution 及其余字段必须带原值一起提交，
    // 否则校验失败或被占位值覆盖；bug-view 返回的 title 可能含 &#039;，提交前还原成 '，
    // 否则禅道会二次转义渲染出字面量
    await updateBug(editBugId.value, {
      title: String(bug.title || '').replace(/&#039;/g, "'"),
      type: bug.type || '',
      severity: Number(bug.severity) || 0,
      pri: Number(bug.pri) || 0,
      steps: bug.steps || '',
      assignedTo: bug.assignedTo?.account || '',
      openedBuild: bug.openedBuild || [],
      execution: bug.execution || 0,
      resolution: String((bug as unknown as { resolution?: unknown }).resolution ?? ''),
      fixedPackageVersion: editForm.fixedPackageVersion,
      reasonAnalysis: editForm.reasonAnalysis,
      advice: editForm.advice,
      implementation: editForm.implementation,
      testAdvise: editForm.testAdvise,
      resolvedBuild: editForm.resolvedBuild,
      comment: editForm.comment
    })
    ElMessage.success('提交成功，已回读刷新，请核对详情确认字段落库')
    editDialogVisible.value = false
    // 回查：接口返回 success 不代表字段落库，以回读为准
    emit('done')
  } catch (err) {
    ElMessage.error(extractApiError(err) || '提交失败')
  } finally {
    editSubmitting.value = false
  }
}

defineExpose({ openAction: handleBugAction, openEdit: openBugEditDialog, openEffort: openBugEffortDialog })
</script>
