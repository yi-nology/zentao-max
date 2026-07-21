<template>
  <div class="product-selector">
    <el-select
      :model-value="selectedProduct"
      placeholder="请选择产品"
      clearable
      filterable
      style="width: 200px"
      @update:model-value="handleProductChange"
    >
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
const selectedProduct = ref<number | string>('')
const selectedProject = ref<number | string>('')

// 加载产品列表（一次性）
const loadProductList = async (): Promise<void> => {
  try {
    const res = await getProducts()
    productOptions.value = res.data || []
  } catch (error) {
    console.error('获取产品列表失败:', error)
  }
}

// 按产品加载项目（联动的核心：选产品时按 productId 重新拉项目列表）
const loadProjectListByProduct = async (productId: number | string): Promise<void> => {
  projectLoading.value = true
  try {
    const res = await getProjects({ productId })
    projectOptions.value = res.data || []
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
