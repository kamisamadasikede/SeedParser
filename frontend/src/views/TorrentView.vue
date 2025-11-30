<script setup lang="ts">
import { ref, inject } from 'vue';
import type { Ref } from 'vue';
import { useRouter } from 'vue-router';
import { ParseTorrentFile ,DownloadTorrentFiles } from "../../wailsjs/go/main/App";
import { ResolveFilePaths, CanResolveFilePaths } from "../../wailsjs/runtime/runtime";

// Theme management - using global theme from App.vue
const currentTheme = inject('currentTheme') as Ref<string>;
// Torrent drop area state
const isDragging = ref(false);
const showResults = ref(false);
const isParsing = ref(false);
const torrentInfo = ref<any>(null);
const errorMessage = ref('');
const selectedFiles = ref<string[]>([]); // 存储选中的文件名称
const torrentFilePath = ref<string>(''); // 存储真实的文件路径
const currentFile = ref<File | null>(null); // 存储当前处理的文件
const router = useRouter();
const isDownloading = ref(false); // 防止重复提交标志

// Handle drag events
const handleDragOver = (event: DragEvent) => {
  event.preventDefault();
  isDragging.value = true;
};

const handleDragLeave = () => {
  isDragging.value = false;
};

const handleDrop = (event: DragEvent) => {
  event.preventDefault();
  isDragging.value = false;
  
  if (event.dataTransfer?.files.length) {
    const file = event.dataTransfer.files[0];
    if (file.name.endsWith('.torrent')) {
      parseTorrentFile(file);
    } else {
      errorMessage.value = '请选择 .torrent 文件';
    }
  }
};

// Handle file input change
const handleFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement;
  if (target.files && target.files.length > 0) {
    const file = target.files[0];
    if (file.name.endsWith('.torrent')) {
      parseTorrentFile(file);
    } else {
      errorMessage.value = '请选择 .torrent 文件';
    }
  }
};

// Parse torrent file
const parseTorrentFile = async (file: File) => {
  try {
    isParsing.value = true;
    errorMessage.value = '';
    
    // 存储当前处理的文件
    currentFile.value = file;
    
    // 读取文件内容为ArrayBuffer
    const arrayBuffer = await file.arrayBuffer();
    // 转换为Uint8Array
    const uint8Array = new Uint8Array(arrayBuffer);
    // 转换为Base64字符串，方便传输
    const base64String = btoa(String.fromCharCode(...uint8Array));
    
    // 存储文件名
    torrentFilePath.value = file.name;
    
    // 调用后端方法，传递Base64编码的文件内容和文件名
    const result = await ParseTorrentFile(JSON.stringify({ 
      content: base64String, 
      fileName: file.name 
    }));
    
    const parsedResult = JSON.parse(result);
    
    // Format the result to match the expected structure
    // 格式化结果以匹配预期结构
    torrentInfo.value = {
      name: parsedResult.fileName, // 使用后端返回的fileName字段
      size: parsedResult.totalSize,
      files: parsedResult.files.map((file: any) => ({
        path: [file.name], // 使用文件名作为路径
        length: file.size, // 使用后端返回的文件大小
        name: file.name
      })),
      infoHash: "",
      createdBy: "",
      creationDate: 0,
      comment: "",
      announce: [],
      announceList: []
    };
    
    showResults.value = true;
  } catch (error) {
    console.error('Error parsing torrent file:', error);
    errorMessage.value = '解析种子文件失败: ' + (error as Error).message;
    
    // Fallback to mock data if backend call fails
    // 如果后端调用失败，使用模拟数据
    const mockTorrent = {
      name: "Sample Torrent",
      size: 1024 * 1024 * 1024, // 1GB
      files: [
        {
          path: ["Sample Video 1.mp4"],
          length: 720 * 1024 * 1024, // 720MB
          name: "Sample Video 1.mp4",
        },
        {
          path: ["Sample Video 2.mp4"],
          length: 512 * 1024 * 1024, // 512MB
          name: "Sample Video 2.mp4",
        },
      ],
      infoHash: "abcdef1234567890abcdef1234567890abcdef12",
      createdBy: "Transmission/2.94",
      creationDate: 1609459200, // 2021-01-01
      comment: "Sample torrent file",
      announce: ["http://tracker.example.com/announce"],
      announceList: [
        ["http://tracker.example.com/announce"],
        ["http://tracker2.example.com/announce"],
      ],
    };
    
    torrentInfo.value = mockTorrent;
    showResults.value = true;
  } finally {
    isParsing.value = false;
  }
};

// Download all files
const downloadAll = async () => {
  try {
    // 防止重复提交
    if (isDownloading.value) {
      return;
    }
    
    if (!currentFile.value) {
      errorMessage.value = '请先解析种子文件';
      return;
    }
    
    // 设置下载中标志
    isDownloading.value = true;
    
    // 读取文件内容为ArrayBuffer
    const arrayBuffer = await currentFile.value.arrayBuffer();
    // 转换为Uint8Array
    const uint8Array = new Uint8Array(arrayBuffer);
    // 转换为Base64字符串，方便传输
    const base64String = btoa(String.fromCharCode(...uint8Array));
    
    // Call backend to download all files
    const result = await DownloadTorrentFiles(JSON.stringify({ 
      content: base64String, 
      fileName: currentFile.value.name 
    }), []);
    const response = JSON.parse(result);
    console.log('Download started:', response);
    
    // 跳转到下载管理页面
    router.push('/downloads');
    
    // 1秒后恢复可提交状态
    setTimeout(() => {
      isDownloading.value = false;
    }, 1000);
  } catch (error) {
    console.error('Error starting download:', error);
    errorMessage.value = '开始下载失败: ' + (error as Error).message;
    
    // 恢复可提交状态
    isDownloading.value = false;
  }
};

// Format file size
const formatFileSize = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};
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
      >种子解析</h2>
      <p 
        :class="{
          'text-gray-400': currentTheme === 'dark',
          'text-gray-500': currentTheme === 'light'
        }"
      >上传或拖放种子文件进行解析</p>
    </div>
    
    <!-- Error Message -->
    <div v-if="errorMessage" 
      class="px-4 py-3 rounded-lg mb-6"
      :class="{
        'bg-red-900 bg-opacity-30 border border-red-700 text-red-300': currentTheme === 'dark',
        'bg-red-100 border border-red-200 text-red-800': currentTheme === 'light'
      }"
    >
      <i class="fa fa-exclamation-circle mr-2"></i>
      {{ errorMessage }}
    </div>
    
    <!-- Torrent Upload Area -->
    <div 
      id="torrent-drop-area" 
      class="torrent-drop-area rounded-lg p-8 mb-8 text-center" 
      :class="{ 
        'drag-over': isDragging,
        'bg-secondary': currentTheme === 'dark',
        'bg-white border border-gray-200': currentTheme === 'light'
      }"
      @dragover="handleDragOver"
      @dragleave="handleDragLeave"
      @drop="handleDrop"
    >
      <div class="flex flex-col items-center justify-center">
        <img src="https://p11-doubao-search-sign.byteimg.com/tos-cn-i-be4g95zd3a/2133098436854808591~tplv-be4g95zd3a-image.jpeg?rk3s=542c0f93&x-expires=1779862928&x-signature=Mhsfe7WDUwKMoPSLiar4xv4OCIw%3D" alt="Torrent Icon" class="w-16 h-16 mb-4 text-accent opacity-70">
        <h3 
          class="text-xl font-semibold mb-2"
          :class="{
            'text-white': currentTheme === 'dark',
            'text-gray-900': currentTheme === 'light'
          }"
        >拖放种子文件到此处</h3>
        <p 
          class="mb-4"
          :class="{
            'text-gray-400': currentTheme === 'dark',
            'text-gray-500': currentTheme === 'light'
          }"
        >或</p>
        <label 
          class="btn-primary bg-accent hover:bg-accentDark text-white py-2 px-6 rounded-lg cursor-pointer"
        >
          <span>{{ isParsing ? '解析中...' : '选择种子文件' }}</span>
          <input 
            type="file" 
            id="torrent-file-input" 
            class="hidden" 
            accept=".torrent"
            @change="handleFileChange"
            :disabled="isParsing"
          >
        </label>
        <p 
          class="text-xs mt-4"
          :class="{
            'text-gray-500': currentTheme === 'dark',
            'text-gray-400': currentTheme === 'light'
          }"
        >支持 .torrent 文件</p>
      </div>
    </div>
    
    <!-- Parsing Indicator -->
    <div v-if="isParsing" 
      class="rounded-lg p-6 shadow-lg mb-8 text-center"
      :class="{
        'bg-secondary': currentTheme === 'dark',
        'bg-white border border-gray-200': currentTheme === 'light'
      }"
    >
      <div class="flex flex-col items-center justify-center">
        <div class="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-accent mb-4"></div>
        <h3 
          class="text-lg font-semibold mb-2"
          :class="{
            'text-white': currentTheme === 'dark',
            'text-gray-900': currentTheme === 'light'
          }"
        >正在解析种子文件...</h3>
        <p 
          :class="{
            'text-gray-400': currentTheme === 'dark',
            'text-gray-500': currentTheme === 'light'
          }"
        >请稍候，正在读取种子信息</p>
      </div>
    </div>
    
    <!-- Torrent Results -->
    <div id="torrent-results" v-if="showResults && torrentInfo" class="fade-in">
      <div 
        class="rounded-lg p-6 shadow-lg mb-8"
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
          >种子信息</h3>
          <span class="text-sm text-accent">解析完成</span>
        </div>
        
        <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
          <div>
            <h4 
              class="text-sm mb-1"
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            >种子名称</h4>
            <p 
              class="font-medium" 
              id="torrent-name"
              :class="{
                'text-white': currentTheme === 'dark',
                'text-gray-900': currentTheme === 'light'
              }"
            >{{ torrentInfo.name }}</p>
          </div>
          <div>
            <h4 
              class="text-sm mb-1"
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            >文件数量</h4>
            <p 
              class="font-medium" 
              id="torrent-file-count"
              :class="{
                'text-white': currentTheme === 'dark',
                'text-gray-900': currentTheme === 'light'
              }"
            >{{ torrentInfo.files.length }}</p>
          </div>
          <div>
            <h4 
              class="text-sm mb-1"
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            >总大小</h4>
            <p 
              class="font-medium" 
              id="torrent-total-size"
              :class="{
                'text-white': currentTheme === 'dark',
                'text-gray-900': currentTheme === 'light'
              }"
            >{{ formatFileSize(torrentInfo.size) }}</p>
          </div>
        </div>
        
        <div 
          class="pt-6"
          :class="{
            'border-t border-gray-700': currentTheme === 'dark',
            'border-t border-gray-200': currentTheme === 'light'
          }"
        >
            <h4 
              class="text-lg font-semibold mb-4"
              :class="{
                'text-white': currentTheme === 'dark',
                'text-gray-900': currentTheme === 'light'
              }"
            >包含的文件</h4>
            
            <div class="overflow-x-auto">
              <table class="w-full text-left">
                <thead>
                  <tr 
                    :class="{
                      'border-b border-gray-700': currentTheme === 'dark',
                      'border-b border-gray-200': currentTheme === 'light'
                    }"
                  >
                    <th 
                      class="pb-3 font-medium"
                      :class="{
                        'text-gray-400': currentTheme === 'dark',
                        'text-gray-500': currentTheme === 'light'
                      }"
                    >文件名</th>
                    <th 
                      class="pb-3 font-medium"
                      :class="{
                        'text-gray-400': currentTheme === 'dark',
                        'text-gray-500': currentTheme === 'light'
                      }"
                    >大小</th>
                    <th 
                      class="pb-3 font-medium"
                      :class="{
                        'text-gray-400': currentTheme === 'dark',
                        'text-gray-500': currentTheme === 'light'
                      }"
                    >类型</th>
                  </tr>
                </thead>
                <tbody id="torrent-file-list">
                  <tr 
                    v-for="file in torrentInfo.files" 
                    :key="file.name"
                    :class="{
                      'border-b hover:bg-gray-800 text-gray-300': currentTheme === 'dark',
                      'border-b border-gray-200 hover:bg-gray-50 text-gray-700': currentTheme === 'light'
                    }"
                  >
                    <td class="py-4">{{ file.name }}</td>
                    <td 
                      class="py-4"
                      :class="{
                        'text-gray-400': currentTheme === 'dark',
                        'text-gray-500': currentTheme === 'light'
                      }"
                    >{{ formatFileSize(file.length) }}</td>
                    <td 
                      class="py-4"
                      :class="{
                        'text-gray-400': currentTheme === 'dark',
                        'text-gray-500': currentTheme === 'light'
                      }"
                    >{{ file.name.split('.').pop()?.toUpperCase() || '未知' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          
          <div class="mt-6 flex justify-end">
            <button 
              id="download-all" 
              class="btn-primary bg-accent hover:bg-accentDark text-white py-2 px-6 rounded-lg flex items-center"
              @click="downloadAll"
              :disabled="isDownloading"
              :class="{ 'opacity-50 cursor-not-allowed': isDownloading }"
            >
              <i class="fa fa-download mr-2"></i>
              <span>{{ isDownloading ? '下载中...' : '下载所有文件' }}</span>
            </button>
          </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
/* Torrent specific styles */
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

.torrent-drop-area:hover, .torrent-drop-area.drag-over {
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