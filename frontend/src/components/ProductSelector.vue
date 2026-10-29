<template>
  <div class="product-selector">
    <el-select
      :model-value="selectedProduct"
      placeholder="请选择产品"
      clearable
      filterable
      :loading="productLoading"
      style="width: 200px"
      @visible-change="onProductDropdownToggle"
      @update:model-value="handleProductChange"
    >
      <el-option v-if="productLoadFailed" label="加载失败，点击重试" value="__retry__" />
      <el-option
        v-for="item in productOptions"
        :key="item.id"
        :label="item.name"
        :value="item.id"
      />
    </el-select>
    <el-select
      :model-value="selectedProject"
      :placeholder="selectedProduct ? '请选择项目' : '请先选择产品'"
      clearable
      filterable
      :disabled="!selectedProduct"
      :loading="projectLoading"
      style="width: 200px"
      @update:model-value="handleProjectChange"
    >
      <el-option
        v-for="item in projectOptions"
        :key="item.id"
        :label="item.name"
        :value="item.id"
      />
    </el-select>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { getProducts, getProjects } from '@/api/zentao'
import type { Product, Project } from '@/types/api'

interface SelectionValue {
  product: number | null
  project: number | null
}

const props = defineProps<{
  modelValue?: SelectionValue
}>()

const emit = defineEmits<{
  'update:modelValue': [value: SelectionValue]
  'change': [value: SelectionValue]
}>()

const productOptions = ref<Product[]>([])
const projectOptions = ref<Project[]>([])
const projectLoading = ref(false)
const productLoading = ref(false)
const productLoadFailed = ref(false)
const selectedProduct = ref<number | string>('')
const selectedProject = ref<number | string>('')

// 加载产品列表（一次性）。冷拉需 10s+（后端遍历禅道分页），失败自动重试——
// 此前静默吞掉，下拉永远"无数据"且无提示无自愈
const loadProductList = async (retry = 0): Promise<void> => {
  productLoading.value = true
  productLoadFailed.value = false
  try {
    const res = await getProducts()
    productOptions.value = res.data || []
    // 脏数据自愈：持久化恢复的 product 不在产品列表里时自动清空，
    // 避免下拉显示裸 ID（Element Plus 对无匹配选项的 value 显示原值）
    if (selectedProduct.value && !productOptions.value.some((p) => p.id === Number(selectedProduct.value))) {
      selectedProduct.value = ''
      selectedProject.value = ''
      projectOptions.value = []
      emitSelection()
    }
  } catch (error) {
    console.error('获取产品列表失败:', error)
    productLoadFailed.value = true
    if (retry < 2) {
      setTimeout(() => loadProductList(retry + 1), 1500 * (retry + 1))
    }
  } finally {
    productLoading.value = false
  }
}

// 打开下拉时若列表为空（首拉失败且重试也失败），兜底重拉
const onProductDropdownToggle = (visible: boolean): void => {
  if (visible && productOptions.value.length === 0 && !productLoading.value) {
    loadProductList()
  }
}

// 按产品加载项目（联动的核心：选产品时按 productId 重新拉项目列表）
const loadProjectListByProduct = async (productId: number | string): Promise<void> => {
  projectLoading.value = true
  try {
    const res = await getProjects({ productId })
    projectOptions.value = res.data || []
    // 脏数据自愈：持久化恢复的 project 不在该产品的项目列表里时自动清空
    if (selectedProject.value && !projectOptions.value.some((p) => p.id === Number(selectedProject.value))) {
      selectedProject.value = ''
      emitSelection()
    }
  } catch (error) {
    console.error('按产品加载项目列表失败:', error)
    projectOptions.value = []
  } finally {
    projectLoading.value = false
  }
}

const emitSelection = (): void => {
  const selection: SelectionValue = {
    product: selectedProduct.value ? Number(selectedProduct.value) : null,
    project: selectedProject.value ? Number(selectedProject.value) : null
  }
  emit('update:modelValue', selection)
  emit('change', selection)
}

// 用户选产品（含清空）
const handleProductChange = async (productId: number | string | null | undefined): Promise<void> => {
  // "加载失败，点击重试" 占位项：触发重拉，不进入选择流程
  if (productId === '__retry__') {
    selectedProduct.value = ''
    loadProductList()
    return
  }
  const newProduct = productId ?? ''
  selectedProduct.value = newProduct
  selectedProject.value = ''
  projectOptions.value = []

  if (newProduct) {
    await loadProjectListByProduct(newProduct)
  }

  emitSelection()
}

const handleProjectChange = (projectId: number | string | null | undefined): void => {
  selectedProject.value = projectId ?? ''
  emitSelection()
}

onMounted(async () => {
  await loadProductList()
  // 从 URL / 外部状态回填 product 时，按 product 联动加载项目
  const initialProduct = props.modelValue?.product
  if (initialProduct) {
    selectedProduct.value = initialProduct
    if (props.modelValue?.project) {
      selectedProject.value = props.modelValue.project
    }
    await loadProjectListByProduct(initialProduct)
  }
})

// 监听外部 modelValue 变化（如 URL 变化）：产品变了 → 重新拉项目列表
watch(() => props.modelValue?.product, async (newProduct) => {
  if (newProduct == null) return
  if (newProduct === selectedProduct.value) return
  selectedProduct.value = newProduct
  selectedProject.value = props.modelValue?.project ?? ''
  await loadProjectListByProduct(newProduct)
})
</script>

<style scoped>
.product-selector {
  display: flex;
  gap: 10px;
}
</style>
