<template>
  <div class="mx-auto">
    <div class="flex justify-between items-center mb-6">
      <h2 class="text-2xl font-bold dark:text-white">文件编辑器</h2>
      <button @click="refreshFiles" class="btn btn-secondary">
        <i class="fas fa-sync-alt"></i> 刷新
      </button>
    </div>

    <div class="card">
      <div class="editor-layout">
        <!-- 左侧文件树 -->
        <div class="file-tree bg-gray-50 dark:bg-gray-800">
          <div class="file-tree-header p-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-medium dark:text-gray-200">账本文件</h3>
          </div>
          <div class="file-tree-content overflow-y-auto" style="max-height: calc(100vh - 300px)">
            <ul class="tree">
              <li v-for="file in files" :key="file" class="tree-item">
                <div 
                  class="file-item px-4 py-3 text-sm font-medium hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                  :class="{ 'bg-primary/10 text-primary dark:bg-primary/20 dark:text-primary': selectedFile === file }"
                  @click="selectFile(file)"
                >
                  <span class="file-icon text-gray-500 dark:text-gray-400 mr-2">📄</span>
                  <span class="truncate">{{ file }}</span>
                </div>
              </li>
            </ul>
          </div>
        </div>
        
        <!-- 右侧编辑器 -->
        <div class="editor-panel flex-1">
          <div class="editor-header p-4 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
            <div class="file-info">
              <h3 class="text-lg font-medium dark:text-gray-200">{{ selectedFile || '选择文件' }}</h3>
            </div>
            <div class="editor-actions flex gap-2">
              <button @click="saveFile" class="btn btn-primary" :disabled="!selectedFile || !contentChanged">
                <i class="fas fa-save"></i> 保存
              </button>
              <button @click="reloadFile" class="btn btn-secondary" :disabled="!selectedFile">
                <i class="fas fa-redo"></i> 重新加载
              </button>
            </div>
          </div>
          
          <div class="editor-content">
            <div v-if="!selectedFile" class="no-file-selected flex flex-col items-center justify-center h-96 text-gray-500 dark:text-gray-400">
              <span class="text-6xl mb-4">📄</span>
              <p>请从左侧选择一个文件进行编辑</p>
            </div>
            <div v-else>
              <textarea 
                v-model="fileContent" 
                class="code-editor w-full h-96 p-4 border-0 focus:ring-0 bg-gray-50 dark:bg-gray-900 text-gray-800 dark:text-gray-200 font-mono text-sm"
                spellcheck="false"
                @input="handleContentChange"
              ></textarea>
            </div>
          </div>
          
          <div class="editor-footer p-4 border-t border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
            <div class="status-message" 
                 :class="statusType === 'success' ? 'text-green-600 dark:text-green-400' : 
                        statusType === 'error' ? 'text-red-600 dark:text-red-400' : ''">
              {{ statusMessage }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '~/composables/useApi'

const { getFiles, getFileContent, saveFileContent } = useApi()

// 状态管理
const files = ref<string[]>([])
const selectedFile = ref<string | null>(null)
const fileContent = ref('')
const originalContent = ref('')
const contentChanged = ref(false)
const statusMessage = ref('')
const statusType = ref('')

// 加载文件列表
const loadFiles = async () => {
  try {
    const data = await getFiles()
    files.value = data.files
  } catch (error) {
    console.error('Failed to load files:', error)
    showStatus('加载文件失败', 'error')
  }
}

// 选择文件并加载内容
const selectFile = async (file: string) => {
  selectedFile.value = file
  try {
    const data = await getFileContent(file)
    fileContent.value = data.content
    originalContent.value = data.content
    contentChanged.value = false
    showStatus('')
  } catch (error) {
    console.error('Failed to load file content:', error)
    showStatus('加载文件内容失败', 'error')
  }
}

// 监听内容变化
const handleContentChange = () => {
  contentChanged.value = fileContent.value !== originalContent.value
}

// 保存文件
const saveFile = async () => {
  if (!selectedFile.value || !contentChanged.value) return
  
  try {
    await saveFileContent(selectedFile.value, fileContent.value)
    originalContent.value = fileContent.value
    contentChanged.value = false
    showStatus('文件保存成功', 'success')
    
    // 通知其他组件账本已更新
    window.dispatchEvent(new CustomEvent('ledger-updated'))
  } catch (error) {
    console.error('Failed to save file:', error)
    showStatus('保存文件失败', 'error')
  }
}

// 重新加载文件
const reloadFile = () => {
  if (selectedFile.value) {
    selectFile(selectedFile.value)
  }
}

// 刷新文件列表
const refreshFiles = () => {
  loadFiles()
}

// 显示状态消息
const showStatus = (message: string, type: string = '') => {
  statusMessage.value = message
  statusType.value = type
  
  // 3秒后清除状态消息
  if (message) {
    setTimeout(() => {
      statusMessage.value = ''
      statusType.value = ''
    }, 3000)
  }
}

// 初始化加载文件列表
onMounted(() => {
  loadFiles()
})
</script>

<style scoped>
.editor-layout {
  display: flex;
  flex-direction: column;
  gap: 0;
}

@media (min-width: 768px) {
  .editor-layout {
    flex-direction: row;
  }

  .file-tree {
    width: 300px;
    border-right: 1px solid #e5e7eb;
  }
}

.file-item {
  cursor: pointer;
}

.code-editor {
  resize: vertical;
  tab-size: 4;
  line-height: 1.6;
}
</style>