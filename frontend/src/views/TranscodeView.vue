<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, inject } from 'vue'
import { GetTranscodeStatus, CancelTranscode, UploadFile, StartTranscode } from '../../wailsjs/go/main/App'

// Theme management - using global theme from App.vue
const currentTheme = inject('currentTheme');
const updateTheme = inject('updateTheme');

// Define the notification function type
const addNotification = inject('addNotification') as (message: string, type: 'success' | 'error' | 'warning' | 'info', duration?: number) => number;

interface TranscodeTask {
  taskId: string
  inputFile: string
  outputFile: string
  status: string
  progress: number
  speed: string
  timeRemaining: string
  startTime: string
  endTime: string
  error: string
  videoCodec: string
  audioCodec: string
  resolution: string
  bitrate: string
  pid: number
}

const tasks = ref<TranscodeTask[]>([])
const selectedFile = ref<File | null>(null)
const outputFormat = ref('mp4')
const resolution = ref('720p')
const quality = ref(7)
const ffmpegParams = ref('-c:v libx264 -c:a aac -preset medium')
const isTranscoding = ref(false)
const isUploading = ref(false)
const uploadedFileName = ref('')
const showTranscodeSettings = ref(false)
const uploadProgress = ref(0) // 上传进度，0-100
let statusInterval: number | null = null

// 选择文件
const handleFileSelect = (event: Event) => {
  const target = event.target as HTMLInputElement
  if (target.files && target.files[0]) {
    selectedFile.value = target.files[0]
  }
}

// 重置文件输入框
const resetFileInput = () => {
  const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
  if (fileInput) {
    fileInput.value = ''
  }
}

// 添加拖放功能
const isDragOver = ref(false)

const handleDragOver = (event: DragEvent) => {
  event.preventDefault()
  isDragOver.value = true
}

const handleDragLeave = (event: DragEvent) => {
  event.preventDefault()
  isDragOver.value = false
}

const handleDrop = (event: DragEvent) => {
  event.preventDefault()
  isDragOver.value = false
  
  const files = event.dataTransfer?.files
  if (files && files.length > 0) {
    const file = files[0]
    
    // 检查文件类型
    const allowedTypes = ['video/mp4', 'video/avi', 'video/mkv', 'video/mov', 'video/quicktime', 'video/x-msvideo']
    if (!allowedTypes.includes(file.type)) {
      alert('不支持的文件格式，请选择 MP4、AVI、MKV、MOV 格式的视频文件')
      return
    }
    
    // 检查文件大小
    const maxSize = 500 * 1024 * 1024 // 500MB
    if (file.size > maxSize) {
      alert(`文件大小超过限制 (最大500MB，当前: ${(file.size / 1024 / 1024).toFixed(2)}MB)`)
      return
    }
    
    selectedFile.value = file
  }
}

// 优化：分块读取文件，避免大文件内存溢出
const fileToBase64 = (file: File): Promise<string> => {
  return new Promise((resolve, reject) => {
    // 限制文件大小，最大500MB
    const maxSize = 500 * 1024 * 1024; // 500MB
    if (file.size > maxSize) {
      reject(new Error(`文件大小超过限制 (最大500MB，当前: ${(file.size / 1024 / 1024).toFixed(2)}MB)`));
      return;
    }
    
    const reader = new FileReader();
    
    reader.onprogress = (e) => {
      if (e.lengthComputable) {
        const progress = Math.round((e.loaded / e.total) * 100);
        uploadProgress.value = Math.min(progress, 90); // 最多90%，保留10%给上传
      }
    };
    
    reader.onload = (e) => {
      try {
        const result = e.target?.result as string;
        const base64 = result.split(',')[1];
        uploadProgress.value = 95;
        resolve(base64);
      } catch (error) {
        reject(new Error('文件读取失败: ' + (error as Error).message));
      }
    };
    
    reader.onerror = () => {
      reject(new Error('文件读取出错'));
    };
    
    // 使用readAsDataURL读取文件
    reader.readAsDataURL(file);
  });
}

// 上传文件
const uploadFile = async () => {
  if (!selectedFile.value) {
    addNotification('请选择视频文件', 'warning')
    return
  }

  // 检查文件类型
  const allowedTypes = ['video/mp4', 'video/avi', 'video/mkv', 'video/mov', 'video/quicktime', 'video/x-msvideo']
  if (!allowedTypes.includes(selectedFile.value.type)) {
    addNotification('不支持的文件格式，请选择 MP4、AVI、MKV、MOV 格式的视频文件', 'error')
    return
  }

  try {
    isUploading.value = true
    uploadProgress.value = 0
    
    console.log('开始上传文件:', selectedFile.value.name, '大小:', (selectedFile.value.size / 1024 / 1024).toFixed(2) + 'MB')
    
    // 转换文件为Base64（已限制文件大小）
    const base64Content = await fileToBase64(selectedFile.value)
    uploadProgress.value = 95
    
    // 构建请求数据
    const requestData = {
      content: base64Content,
      fileName: selectedFile.value.name
    }

    // 减少超时时间到15秒，避免长时间等待
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 15000) // 15秒超时
    
    try {
      const response = await UploadFile(JSON.stringify(requestData))
      clearTimeout(timeoutId)
      
      const result = JSON.parse(response)
      console.log('文件上传成功:', result)
      
      uploadProgress.value = 100
      uploadedFileName.value = selectedFile.value.name
      showTranscodeSettings.value = true
      
      // 清空选择
      selectedFile.value = null
      resetFileInput()
      
    } catch (uploadError) {
      clearTimeout(timeoutId)
      throw uploadError
    }
    
  } catch (error) {
    console.error('文件上传失败:', error)
    const errorMessage = (error as Error).message
    
    if (errorMessage.includes('AbortError') || errorMessage.includes('超时')) {
      alert('文件上传超时，请重试')
    } else if (errorMessage.includes('文件大小超过限制')) {
      alert(errorMessage)
    } else {
      alert('文件上传失败: ' + errorMessage)
    }
    
    uploadProgress.value = 0
    isUploading.value = false
  } finally {
    // 延迟重置状态，给用户时间看到结果
    setTimeout(() => {
      isUploading.value = false
      if (uploadProgress.value === 100) {
        // 成功上传后保留100%显示2秒，然后重置
        setTimeout(() => {
          uploadProgress.value = 0
        }, 2000)
      } else {
        uploadProgress.value = 0
      }
    }, 500)
  }
}

// 开始转码
const startTranscode = async () => {
  if (!uploadedFileName.value) {
    alert('请先上传文件')
    return
  }

  try {
    // 构建请求数据
    const requestData = {
      fileName: uploadedFileName.value,
      outputFormat: outputFormat.value,
      resolution: resolution.value,
      quality: quality.value,
      videoCodec: 'libx264',
      audioCodec: 'aac',
      ffmpegParams: ffmpegParams.value
    }

    // 显示任务已提交通知
    addNotification('转码任务已提交，请查看任务列表', 'success');
    
    // 立即重置所有上传相关状态
    showTranscodeSettings.value = false
    uploadedFileName.value = ''
    selectedFile.value = null
    uploadProgress.value = 0
    
    // 重置文件输入框，允许选择相同的文件
    resetFileInput()
    
    // 重置转码设置为默认值
    outputFormat.value = 'mp4'
    resolution.value = '720p'
    quality.value = 7
    ffmpegParams.value = '-c:v libx264 -c:a aac -preset medium'
    
    // 后台异步提交转码任务，不阻塞UI
    StartTranscode(JSON.stringify(requestData))
      .then(response => {
        const result = JSON.parse(response)
        console.log('转码任务后台提交成功:', result)
      })
      .catch(error => {
        console.error('后台提交转码任务失败:', error)
        // 可以在这里显示一个非阻塞的通知
      })
    
  } catch (error) {
    console.error('添加转码任务失败:', error)
    alert('添加转码任务失败: ' + (error as Error).message)
  }
}

// 加载转码任务列表
const loadTranscodeTasks = async () => {
  try {
    // 调用后端API获取转码任务列表
    const response = await GetTranscodeStatus('')
    const result = JSON.parse(response)
    tasks.value = result.tasks || []
    
    // 检查是否有正在转码的任务
    isTranscoding.value = tasks.value.some(task => task.status === 'transcoding')
    
  } catch (error) {
    console.error('加载转码任务失败:', error)
  }
}

// 取消转码任务
const cancelTranscode = async (taskId: string) => {
  try {
    // 调用后端API取消转码任务
    const response = await CancelTranscode(taskId)
    const result = JSON.parse(response)
    
    if (result.status === 'success') {
      await loadTranscodeTasks()
    } else {
      throw new Error(result.message || '取消转码任务失败')
    }
    
  } catch (error) {
    console.error('取消转码任务失败:', error)
    alert('取消转码任务失败: ' + (error as Error).message)
  }
}

// 下载转码文件
const downloadTranscodeFile = async (filePath: string) => {
  try {
    console.log('开始下载文件:', filePath)
    
    // 构建文件URL：将绝对路径转换为相对URL
    // 例如：将 "transcode\\file.mp4" 转换为 "/transcode/file.mp4"
    const relativePath = filePath.replace(/\\/g, '/')
    const fileUrl = '/' + relativePath
    
    // 获取文件名
    const fileName = relativePath.split('/').pop()
    if (!fileName) {
      throw new Error('无法获取文件名')
    }
    
    // 使用fetch API下载文件
    const response = await fetch(fileUrl)
    if (!response.ok) {
      throw new Error('文件下载失败')
    }
    
    // 将响应转换为Blob
    const blob = await response.blob()
    
    // 创建下载链接
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = fileName
    
    // 触发下载
    document.body.appendChild(a)
    a.click()
    
    // 清理资源
    setTimeout(() => {
      document.body.removeChild(a)
      window.URL.revokeObjectURL(url)
    }, 100)
    
    console.log('文件下载成功:', fileName)
    
  } catch (error) {
    console.error('文件下载失败:', error)
    alert('文件下载失败: ' + (error as Error).message)
  }
}

// 获取状态显示文本和样式
const getStatusInfo = (status: string) => {
  switch (status) {
    case 'waiting':
      return { text: '等待中', class: 'text-yellow-400' }
    case 'transcoding':
      return { text: '转码中', class: 'text-blue-400' }
    case 'completed':
      return { text: '已完成', class: 'text-green-400' }
    case 'failed':
      return { text: '失败', class: 'text-red-400' }
    case 'cancelled':
      return { text: '已取消', class: 'text-gray-400' }
    default:
      return { text: status, class: 'text-gray-400' }
  }
}

// 格式化文件大小
const formatFileSize = (bytes: number) => {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 格式化时间
const formatTime = (timeString: string) => {
  if (!timeString) return ''
  const date = new Date(timeString)
  return date.toLocaleString('zh-CN')
}

// 获取任务数量文本
const getTaskCountText = () => {
  const count = tasks.value.length
  return count === 0 ? '当前没有任务' : `当前有 ${count} 个任务`
}

// 排序后的转码任务列表，正在转码的任务排在前面
const sortedTasks = computed(() => {
  return [...tasks.value].sort((a, b) => {
    // 正在转码的任务排在最前面
    if (a.status === 'transcoding' && b.status !== 'transcoding') {
      return -1
    }
    if (a.status !== 'transcoding' && b.status === 'transcoding') {
      return 1
    }
    // 等待中的任务排在第二位
    if (a.status === 'waiting' && b.status !== 'waiting') {
      return -1
    }
    if (a.status !== 'waiting' && b.status === 'waiting') {
      return 1
    }
    // 其他状态按开始时间倒序排序（最新的任务排在前面）
    const timeA = new Date(a.startTime).getTime()
    const timeB = new Date(b.startTime).getTime()
    return timeB - timeA
  })
})

onMounted(() => {
  loadTranscodeTasks()
  // 定期刷新状态，每秒刷新一次
  statusInterval = window.setInterval(loadTranscodeTasks, 1000)
})

onUnmounted(() => {
  if (statusInterval) {
    clearInterval(statusInterval)
  }
})
</script>

<template>
  <section class="section-content fade-in">
    <div class="mb-6">
      <h2 
        class="text-2xl font-bold mb-2"
        :class="{
          'text-white': currentTheme === 'dark',
          'text-gray-900': currentTheme === 'light'
        }"
      >视频转码</h2>
      <p 
        :class="{
          'text-gray-400': currentTheme === 'dark',
          'text-gray-500': currentTheme === 'light'
        }"
      >将视频转换为不同格式和分辨率</p>
    </div>
    
    <!-- Transcode Upload Area -->
    <div 
      class="rounded-lg p-8 mb-8"
      :class="{
        'bg-secondary': currentTheme === 'dark',
        'bg-white border border-gray-200': currentTheme === 'light'
      }"
    >
      <div class="flex flex-col md:flex-row items-center justify-between gap-6">
        <div class="flex-1">
          <h3 
            class="text-xl font-semibold mb-4"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >选择视频文件</h3>
          <div 
            class="torrent-drop-area rounded-lg p-6 text-center transition-all duration-300"
            :class="{
              'bg-gray-700 border-2 border-dashed border-accent': isDragOver,
              'border-2 border-dashed border-gray-600': !isDragOver && currentTheme === 'dark',
              'border-2 border-dashed border-gray-300': !isDragOver && currentTheme === 'light',
              'bg-secondary': !isDragOver
            }"
            @dragover="handleDragOver"
            @dragleave="handleDragLeave"
            @drop="handleDrop"
          >
            <div class="flex flex-col items-center justify-center">
              <i 
                class="text-4xl mb-4"
                :class="{
                  'fa fa-upload text-accent': isDragOver,
                  'fa fa-file-video-o text-accent': !isDragOver
                }"
              ></i>
              <h4 
                class="text-lg font-semibold mb-2"
                :class="{
                  'text-white': currentTheme === 'dark',
                  'text-gray-900': currentTheme === 'light'
                }"
              >
                {{ isDragOver ? '释放文件开始上传' : '拖放视频文件到此处' }}
              </h4>
              <p 
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
                class="mb-4"
              >或</p>
              <label class="btn-primary bg-accent hover:bg-accentDark text-white py-2 px-6 rounded-lg cursor-pointer transition-all duration-200 hover:scale-105">
                <span>选择视频文件</span>
                <input type="file" class="hidden" accept="video/*" @change="handleFileSelect">
              </label>
              <p 
                :class="{
                  'text-gray-500': currentTheme === 'dark',
                  'text-gray-400': currentTheme === 'light'
                }"
                class="text-xs mt-4"
              >支持 MP4, MKV, AVI, MOV 等格式 • 最大 500MB</p>
              
              <div v-if="selectedFile" class="mt-4 p-3 rounded-lg text-sm"
                :class="{
                  'bg-gray-700': currentTheme === 'dark',
                  'bg-gray-100 border border-gray-200': currentTheme === 'light'
                }"
              >
                <div 
                  :class="{
                    'text-white font-medium': currentTheme === 'dark',
                    'text-gray-900 font-medium': currentTheme === 'light'
                  }"
                >{{ selectedFile.name }}</div>
                <div 
                  :class="{
                    'text-gray-400 text-xs': currentTheme === 'dark',
                    'text-gray-500 text-xs': currentTheme === 'light'
                  }"
                >{{ formatFileSize(selectedFile.size) }}</div>
              </div>
              
              <div v-if="isUploading" class="mt-4">
                <div class="p-3 bg-yellow-700 rounded-lg text-sm flex items-center mb-2">
                  <i class="fa fa-spinner fa-spin mr-2"></i>
                  <span class="text-white">文件上传中...</span>
                  <span class="ml-auto text-white font-medium">{{ uploadProgress }}%</span>
                </div>
                <!-- 上传进度条 -->
                <div 
                  class="w-full rounded-full h-2.5"
                  :class="{
                    'bg-gray-700': currentTheme === 'dark',
                    'bg-gray-200': currentTheme === 'light'
                  }"
                >
                  <div 
                    class="bg-accent h-2.5 rounded-full transition-all duration-300 ease-in-out"
                    :style="{ width: uploadProgress + '%' }"
                  ></div>
                </div>
              </div>
              
              <div v-if="uploadedFileName" class="mt-4 p-3 bg-green-700 rounded-lg text-sm flex items-center">
                <i class="fa fa-check mr-2"></i>
                <span class="text-white">文件上传成功: {{ uploadedFileName }}</span>
              </div>
              
              <div class="mt-4">
                <button 
                  @click="uploadFile" 
                  :disabled="!selectedFile || isUploading" 
                  class="btn-primary bg-accent hover:bg-accentDark text-white py-2 px-6 rounded-lg flex items-center disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <i class="fa fa-upload mr-2"></i>
                  <span>{{ isUploading ? '上传中...' : '上传文件' }}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
        
        <div class="flex-1" v-if="showTranscodeSettings">
          <h3 
            class="text-xl font-semibold mb-4"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >转码设置</h3>
          <div class="space-y-4">
            <div>
              <label 
                class="block text-sm font-medium mb-2"
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >输出格式</label>
              <select v-model="outputFormat" 
                class="w-full rounded-lg py-2 pl-4 pr-10 appearance-none focus:outline-none focus:ring-2 focus:ring-accent"
                :class="{
                  'bg-gray-700 text-white': currentTheme === 'dark',
                  'bg-gray-100 text-gray-900 border border-gray-200': currentTheme === 'light'
                }"
              >
                <option value="mp4">MP4 (推荐)</option>
                <option value="mkv">MKV</option>
                <option value="avi">AVI</option>
                <option value="webm">WebM</option>
              </select>
            </div>
            
            <div>
              <label 
                class="block text-sm font-medium mb-2"
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >分辨率</label>
              <select v-model="resolution" 
                class="w-full rounded-lg py-2 pl-4 pr-10 appearance-none focus:outline-none focus:ring-2 focus:ring-accent"
                :class="{
                  'bg-gray-700 text-white': currentTheme === 'dark',
                  'bg-gray-100 text-gray-900 border border-gray-200': currentTheme === 'light'
                }"
              >
                <option value="original">原始分辨率</option>
                <option value="1080p">1080p (1920x1080)</option>
                <option value="720p">720p (1280x720)</option>
                <option value="480p">480p (854x480)</option>
                <option value="360p">360p (640x360)</option>
              </select>
            </div>
            
            <div>
              <label 
                class="block text-sm font-medium mb-2"
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >质量</label>
              <div class="flex items-center space-x-4">
                <div class="flex-1">
                  <input v-model="quality" type="range" min="1" max="10" 
                    class="w-full h-2 rounded-lg appearance-none cursor-pointer"
                    :class="{
                      'bg-gray-700': currentTheme === 'dark',
                      'bg-gray-200': currentTheme === 'light'
                    }"
                  >
                </div>
                <span 
                  class="text-sm"
                  :class="{
                    'text-white': currentTheme === 'dark',
                    'text-gray-900': currentTheme === 'light'
                  }"
                >{{ quality }}/10</span>
              </div>
            </div>
            
            <div>
              <label 
                class="block text-sm font-medium mb-2"
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >FFmpeg转码参数</label>
              <input 
                v-model="ffmpegParams" 
                type="text" 
                class="w-full rounded-lg py-2 pl-4 pr-4 focus:outline-none focus:ring-2 focus:ring-accent"
                :class="{
                  'bg-gray-700 text-white': currentTheme === 'dark',
                  'bg-gray-100 text-gray-900 border border-gray-200': currentTheme === 'light'
                }"
                placeholder="例如: -c:v libx264 -c:a aac -preset medium"
              >
            </div>
            
            <div class="flex justify-end">
              <button @click="startTranscode" class="btn-primary bg-accent hover:bg-accentDark text-white py-2 px-6 rounded-lg flex items-center">
                <i class="fa fa-play mr-2"></i>
                <span>开始转码</span>
              </button>
            </div>
          </div>
        </div>
        
        <div class="flex-1" v-else-if="!selectedFile && !uploadedFileName">
          <div class="flex flex-col items-center justify-center h-full">
            <i class="fa fa-cog text-4xl mb-4" 
              :class="{
                'text-gray-500': currentTheme === 'dark',
                'text-gray-400': currentTheme === 'light'
              }"
            ></i>
            <p 
              :class="{
                'text-gray-500': currentTheme === 'dark',
                'text-gray-600': currentTheme === 'light'
              }"
            >上传文件后显示转码设置</p>
          </div>
        </div>
      </div>
    </div>
    
    <!-- Transcode Queue -->
    <div 
      class="rounded-lg p-6 shadow-lg"
      :class="{
        'bg-secondary': currentTheme === 'dark',
        'bg-white border border-gray-200': currentTheme === 'light'
      }"
    >
      <div class="flex items-center justify-between mb-6">
        <h3 
          class="text-xl font-semibold"
          :class="{
            'text-white': currentTheme === 'dark',
            'text-gray-900': currentTheme === 'light'
          }"
        >转码队列</h3>
        <span 
          class="text-sm"
          :class="{
            'text-gray-400': currentTheme === 'dark',
            'text-gray-500': currentTheme === 'light'
          }"
        >{{ getTaskCountText() }}</span>
      </div>
      
      <div class="space-y-4">
        <!-- Transcode item -->
        <div 
          v-for="task in sortedTasks" 
          :key="task.taskId" 
          class="transcode-item pb-4"
          :class="{
            'border-b border-gray-700': currentTheme === 'dark',
            'border-b border-gray-200': currentTheme === 'light'
          }"
        >
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center">
              <div 
                class="w-10 h-10 rounded flex items-center justify-center mr-3"
                :class="{
                  'bg-gray-700': currentTheme === 'dark',
                  'bg-gray-100': currentTheme === 'light'
                }"
              >
                <i class="fa fa-file-video-o text-accent"></i>
              </div>
              <div>
                <h4 
                  class="font-medium"
                  :class="{
                    'text-white': currentTheme === 'dark',
                    'text-gray-900': currentTheme === 'light'
                  }"
                >{{ (task.inputFile || '').split('\\').pop() || (task.inputFile || '未知文件') }}</h4>
                <div 
                  class="flex items-center text-xs"
                  :class="{
                    'text-gray-400': currentTheme === 'dark',
                    'text-gray-500': currentTheme === 'light'
                  }"
                >
                  <span>输出格式: {{ (task.outputFile || '').split('.').pop()?.toUpperCase() || '未知' }}</span>
                  <span class="mx-2">•</span>
                  <span>分辨率: {{ task.resolution || '未知' }}</span>
                </div>
              </div>
            </div>
            <div :class="getStatusInfo(task.status).class" class="text-sm">
              <i class="fa fa-clock-o mr-1"></i>
              {{ getStatusInfo(task.status).text }}
            </div>
          </div>
          
          <div 
            v-if="task.status === 'transcoding'" 
            class="w-full rounded-full h-2.5 mb-2"
            :class="{
              'bg-gray-700': currentTheme === 'dark',
              'bg-gray-200': currentTheme === 'light'
            }"
          >
            <div class="bg-accent h-2.5 rounded-full progress-bar transition-all duration-300" :style="{ width: (task.progress * 100) + '%' }"></div>
          </div>
          
          <div class="flex items-center justify-between text-sm">
            <div 
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            >
              <span v-if="task.status === 'transcoding'">进度: {{ Math.round(task.progress *100) }}%</span>
              <span v-if="task.status === 'transcoding'" class="mx-2">•</span>
              <span v-if="task.speed">速度: {{ task.speed }}</span>
              <span v-if="task.timeRemaining" class="mx-2">•</span>
              <span v-if="task.timeRemaining">剩余: {{ task.timeRemaining }}</span>
            </div>
            <div class="flex space-x-2">
              <button 
                v-if="task.status === 'transcoding'" 
                @click="cancelTranscode(task.taskId)" 
                title="取消"
                :class="{
                  'text-gray-400 hover:text-white': currentTheme === 'dark',
                  'text-gray-500 hover:text-gray-900': currentTheme === 'light'
                }"
              >
                <i class="fa fa-times"></i>
              </button>
              <button 
                v-if="task.status === 'completed'" 
                @click="downloadTranscodeFile(task.outputFile)" 
                title="下载"
                :class="{
                  'text-gray-400 hover:text-white': currentTheme === 'dark',
                  'text-gray-500 hover:text-gray-900': currentTheme === 'light'
                }"
              >
                <i class="fa fa-download"></i>
              </button>
            </div>
          </div>
          
          <div v-if="task.error" class="mt-2 text-xs text-red-400">
            错误: {{ task.error }}
          </div>
        </div>
        
        <div 
          v-if="tasks.length === 0" 
          class="text-center py-8"
          :class="{
            'text-gray-400': currentTheme === 'dark',
            'text-gray-500': currentTheme === 'light'
          }"
        >
          <i class="fa fa-inbox text-4xl mb-4"></i>
          <p>暂无转码任务</p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
/* Transcode specific styles */
.fade-in {
  animation: fadeIn 0.5s ease-in-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.torrent-drop-area {
  border: 2px dashed rgba(16, 185, 129, 0.5);
  transition: all 0.3s ease;
}

.torrent-drop-area:hover {
  border-color: #10b981;
  background-color: rgba(16, 185, 129, 0.05);
}

.btn-primary {
  transition: all 0.2s ease;
}

.btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3);
}
</style>