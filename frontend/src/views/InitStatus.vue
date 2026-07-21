<template>
  <div class="init-status">
    <div class="init-status-container">
      <h1>初始化状态</h1>
      <div v-if="loading" class="loading">
        <div class="spinner"></div>
        <p>正在检查初始化状态...</p>
      </div>
      <div v-else class="status-content">
        <div v-if="initStatus.firstStart" class="status-card first-start">
          <h2>首次启动</h2>
          <p>系统正在进行首次初始化...</p>
          <div v-if="initStatus.status === 'success'" class="status success">
            <i class="icon success"></i>
            <p>初始化成功！</p>
            <p>认证信息已存储到数据库中。</p>
          </div>
          <div v-else-if="initStatus.status === 'error'" class="status error">
            <i class="icon error"></i>
            <p>初始化失败！</p>
            <p>{{ initStatus.message }}</p>
          </div>
          <div v-else class="status pending">
            <i class="icon pending"></i>
            <p>初始化进行中...</p>
          </div>
        </div>
        <div v-else class="status-card normal-start">
          <h2>正常启动</h2>
          <p>系统已完成初始化，从数据库加载配置。</p>
          <div class="status success">
            <i class="icon success"></i>
            <p>系统运行正常</p>
          </div>
        </div>
        <div class="actions">
          <router-link to="/" class="btn">返回首页</router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getInitStatus } from '@/api/zentao'

interface InitStatus {
  firstStart: boolean
  status: 'pending' | 'success' | 'error'
  message: string
}

const loading = ref<boolean>(true)
const initStatus = ref<InitStatus>({
  firstStart: false,
  status: 'pending',
  message: ''
})

const checkInitStatus = async (): Promise<void> => {
  try {
    const res = await getInitStatus()
    if (res?.data) {
      initStatus.value = {
        firstStart: res.data.isFirstStart ?? true,
        status: 'success',
        message: res.data.message ?? ''
      }
    }
  } catch (error) {
    initStatus.value = {
      firstStart: true,
      status: 'error',
      message: '检查初始化状态失败'
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  checkInitStatus()
})
</script>

<style scoped>
.init-status {
  min-height: 100vh;
  display: flex;
  justify-content: center;
  align-items: center;
  background-color: #f5f5f5;
  padding: 20px;
}

.init-status-container {
  background-color: white;
  border-radius: 8px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
  padding: 40px;
  max-width: 600px;
  width: 100%;
  text-align: center;
}

h1 {
  color: #333;
  margin-bottom: 30px;
  font-size: 24px;
}

.loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 4px solid #f3f3f3;
  border-top: 4px solid #3498db;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.status-content {
  display: flex;
  flex-direction: column;
  gap: 30px;
}

.status-card {
  padding: 30px;
  border-radius: 8px;
  background-color: #f9f9f9;
}

.status-card h2 {
  color: #333;
  margin-bottom: 20px;
  font-size: 20px;
}

.status-card p {
  color: #666;
  margin-bottom: 15px;
  line-height: 1.5;
}

.status {
  margin-top: 20px;
  padding: 20px;
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.status.success {
  background-color: #d4edda;
  color: #155724;
}

.status.error {
  background-color: #f8d7da;
  color: #721c24;
}

.status.pending {
  background-color: #d1ecf1;
  color: #0c5460;
}

.icon {
  font-size: 48px;
  font-weight: bold;
}

.icon.success::before {
  content: '✓';
}

.icon.error::before {
  content: '✗';
}

.icon.pending::before {
  content: '⏳';
}

.actions {
  margin-top: 20px;
}

.btn {
  display: inline-block;
  padding: 12px 24px;
  background-color: #4CAF50;
  color: white;
  text-decoration: none;
  border-radius: 4px;
  font-weight: bold;
  transition: background-color 0.3s ease;
}

.btn:hover {
  background-color: #45a049;
}

.first-start {
  border-left: 4px solid #4CAF50;
}

.normal-start {
  border-left: 4px solid #2196F3;
}
</style>
