<script setup lang="ts">
import { ref, onMounted, onUnmounted, inject } from 'vue';
import { GetDownloadStatus, CancelDownload, DownloadTorrentFiles, StartWaitingTask } from '../../wailsjs/go/main/App';

// Theme management - using global theme from App.vue
const currentTheme = inject('currentTheme');
const updateTheme = inject('updateTheme');

// Download tabs state
const activeTab = ref('active');

// Download tasks data
interface DownloadTask {
  taskId: string;
  status: string;
  fileName: string;
  downloaded: number;
  totalSize: number;
  speed: number;
  percentage: number;
  lastUpdate?: string;
  pid?: number;
  magnetLink?: string;
  selectedFiles?: string[];
  outputDir?: string;
  startTime?: string;
  endTime?: string;
}

const downloadTasks = ref<DownloadTask[]>([]);
let updateInterval: number | null = null;

// Format file size to human readable format
const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

// Format speed to human readable format
const formatSpeed = (bytesPerSecond: number): string => {
  if (bytesPerSecond === 0) return '0 B/s';
  const k = 1024;
  const speeds = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
  const i = Math.floor(Math.log(bytesPerSecond) / Math.log(k));
  return parseFloat((bytesPerSecond / Math.pow(k, i)).toFixed(2)) + ' ' + speeds[i];
};

// Calculate ETA in seconds
const calculateETA = (downloaded: number, totalSize: number, speed: number): number => {
  if (speed === 0 || downloaded >= totalSize) return 0;
  const remaining = totalSize - downloaded;
  return Math.floor(remaining / speed);
};

// Format ETA to human readable format
const formatETA = (seconds: number): string => {
  if (seconds === 0) return '0 秒';
  if (seconds < 60) return seconds + ' 秒';
  if (seconds < 3600) return Math.floor(seconds / 60) + ' 分钟';
  return Math.floor(seconds / 3600) + ' 小时 ' + Math.floor((seconds % 3600) / 60) + ' 分钟';
};

// Get download status from backend
const getDownloadStatus = async () => {
  try {
    // Call Wails backend API
    const result = await GetDownloadStatus('');
    const data = JSON.parse(result);
    
    if (data.tasks) {
      downloadTasks.value = data.tasks as DownloadTask[];
    }
  } catch (error) {
    console.error('Failed to get download status:', error);
  }
};

// Switch tabs
const switchTab = (tab: string) => {
  activeTab.value = tab;
};

// Filter tasks by status
const getTasksByStatus = (status: string): DownloadTask[] => {
  switch (status) {
    case 'active':
      return downloadTasks.value.filter(task => task.status === 'downloading');
    case 'completed':
      return downloadTasks.value.filter(task => task.status === 'completed');
    case 'paused':
      return downloadTasks.value.filter(task => task.status === 'paused');
    case 'cancelled':
      return downloadTasks.value.filter(task => task.status === 'cancelled');
    case 'waiting':
      return downloadTasks.value.filter(task => task.status === 'waiting');
    default:
      return [];
  }
};

// Cancel download
const cancelDownload = async (taskId: string) => {
  try {
    await CancelDownload(taskId);
    // Refresh download status
    await getDownloadStatus();
  } catch (error) {
    console.error('Failed to cancel download:', error);
  }
};

// Start waiting download
const startWaitingTask = async (taskId: string) => {
  try {
    await StartWaitingTask(taskId);
    // Refresh download status
    await getDownloadStatus();
  } catch (error) {
    console.error('Failed to start waiting download:', error);
    alert('启动等待任务失败: ' + (error as Error).message);
  }
};

// Restart download
const restartDownload = async (task: DownloadTask) => {
  try {
    // Create a mock file data object similar to what would be sent from the torrent parser
    const fileData = JSON.stringify({
      content: '', // This would normally be the base64 encoded torrent file
      fileName: task.fileName
    });
    
    // Start the download again
    await DownloadTorrentFiles(fileData, task.selectedFiles || []);
    // Refresh download status
    await getDownloadStatus();
  } catch (error) {
    console.error('Failed to restart download:', error);
  }
};

// Lifecycle hooks
onMounted(() => {
  // Initial call to get download status
  getDownloadStatus();
  // Set interval to update status every second
  updateInterval = window.setInterval(getDownloadStatus, 1000);
});

onUnmounted(() => {
  // Clear interval when component is unmounted
  if (updateInterval) {
    clearInterval(updateInterval);
  }
});
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
      >下载管理</h2>
      <p 
        :class="{
          'text-gray-400': currentTheme === 'dark',
          'text-gray-500': currentTheme === 'light'
        }"
      >管理当前和已完成的下载任务</p>
    </div>
    
    <!-- Download Tabs -->
    <div 
      class="rounded-lg shadow-lg mb-8"
      :class="{
        'bg-secondary': currentTheme === 'dark',
        'bg-white border border-gray-200': currentTheme === 'light'
      }"
    >
      <div 
        :class="{
          'border-b border-gray-700': currentTheme === 'dark',
          'border-b border-gray-200': currentTheme === 'light'
        }"
      >
        <div class="flex">
          <button 
            class="download-tab px-6 py-3 font-medium" 
            :class="{
              'text-white border-b-2 border-accent': activeTab === 'active',
              'text-gray-400 hover:text-white': activeTab !== 'active' && currentTheme === 'dark',
              'text-gray-500 hover:text-gray-900': activeTab !== 'active' && currentTheme === 'light'
            }" 
            @click="switchTab('active')"
          >
            正在下载
          </button>
          <button 
            class="download-tab px-6 py-3 font-medium" 
            :class="{
              'text-white border-b-2 border-accent': activeTab === 'waiting',
              'text-gray-400 hover:text-white': activeTab !== 'waiting' && currentTheme === 'dark',
              'text-gray-500 hover:text-gray-900': activeTab !== 'waiting' && currentTheme === 'light'
            }" 
            @click="switchTab('waiting')"
          >
            等待中
          </button>
          <button 
            class="download-tab px-6 py-3 font-medium" 
            :class="{
              'text-white border-b-2 border-accent': activeTab === 'completed',
              'text-gray-400 hover:text-white': activeTab !== 'completed' && currentTheme === 'dark',
              'text-gray-500 hover:text-gray-900': activeTab !== 'completed' && currentTheme === 'light'
            }" 
            @click="switchTab('completed')"
          >
            已完成
          </button>

          <button 
            class="download-tab px-6 py-3 font-medium" 
            :class="{
              'text-white border-b-2 border-accent': activeTab === 'cancelled',
              'text-gray-400 hover:text-white': activeTab !== 'cancelled' && currentTheme === 'dark',
              'text-gray-500 hover:text-gray-900': activeTab !== 'cancelled' && currentTheme === 'light'
            }" 
            @click="switchTab('cancelled')"
          >
            已取消
          </button>
        </div>
      </div>
      
      <!-- Active Downloads -->
      <div id="active-downloads" class="download-content p-6" v-if="activeTab === 'active'">
        <div id="no-active-downloads" class="text-center py-10" v-if="getTasksByStatus('active').length === 0">
          <div 
            class="w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4"
            :class="{
              'bg-gray-700': currentTheme === 'dark',
              'bg-gray-100': currentTheme === 'light'
            }"
          >
            <i class="fa fa-download text-2xl" :class="{
              'text-gray-500': currentTheme === 'dark',
              'text-gray-400': currentTheme === 'light'
            }"></i>
          </div>
          <h3 
            class="text-lg font-semibold mb-2"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >没有正在进行的下载</h3>
          <p 
            class="mb-4"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >点击"种子解析"开始新的下载任务</p>
          <router-link to="/torrent" class="btn-primary bg-accent hover:bg-accentDark text-white py-2 px-6 rounded-lg">
            解析种子
          </router-link>
        </div>
        
        <div id="active-downloads-list" v-else>
          <!-- Download items -->
          <div 
            v-for="task in getTasksByStatus('active')" 
            :key="task.taskId" 
            class="download-item pb-6 mb-6"
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
                  <i class="fa fa-film text-accent"></i>
                </div>
                <div>
                  <h4 
                    class="font-medium"
                    :class="{
                      'text-white': currentTheme === 'dark',
                      'text-gray-900': currentTheme === 'light'
                    }"
                  >{{ task.fileName }}</h4>
                  <div 
                    class="flex items-center text-xs"
                    :class="{
                      'text-gray-400': currentTheme === 'dark',
                      'text-gray-500': currentTheme === 'light'
                    }"
                  >
                    <span>来源: {{ task.fileName }}</span>
                  </div>
                </div>
              </div>
              <div 
                class="text-sm"
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >
                {{ formatFileSize(task.downloaded) }} / {{ formatFileSize(task.totalSize) }}
              </div>
            </div>
            
            <div 
              class="w-full rounded-full h-2.5 mb-2"
              :class="{
                'bg-gray-700': currentTheme === 'dark',
                'bg-gray-200': currentTheme === 'light'
              }"
            >
              <div 
                class="bg-accent h-2.5 rounded-full progress-bar" 
                :style="{ width: `${task.percentage}%` }"
              ></div>
            </div>
            
            <div class="flex items-center justify-between text-sm">
              <div 
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >
                <span>进度: {{ Math.round(task.percentage) }}%</span>
                <span class="mx-2">•</span>
                <span>速度: {{ formatSpeed(task.speed) }}</span>
                <span class="mx-2">•</span>
                <span>剩余: {{ formatETA(calculateETA(task.downloaded, task.totalSize, task.speed)) }}</span>
              </div>
              <div class="flex space-x-2">
                <button 
                  title="取消" 
                  @click="cancelDownload(task.taskId)"
                  :class="{
                    'text-gray-400 hover:text-white': currentTheme === 'dark',
                    'text-gray-500 hover:text-gray-900': currentTheme === 'light'
                  }"
                >
                  <i class="fa fa-times"></i>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- Waiting Downloads -->
      <div id="waiting-downloads" class="download-content p-6" v-if="activeTab === 'waiting'">
        <div id="no-waiting-downloads" class="text-center py-10" v-if="getTasksByStatus('waiting').length === 0">
          <div 
            class="w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4"
            :class="{
              'bg-gray-700': currentTheme === 'dark',
              'bg-gray-100': currentTheme === 'light'
            }"
          >
            <i class="fa fa-clock-o text-2xl" :class="{
              'text-gray-500': currentTheme === 'dark',
              'text-gray-400': currentTheme === 'light'
            }"></i>
          </div>
          <h3 
            class="text-lg font-semibold mb-2"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >没有等待中的下载</h3>
          <p 
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >等待中的下载任务将显示在这里</p>
        </div>
        
        <div id="waiting-downloads-list" v-else>
          <!-- Waiting download items -->
          <div 
            v-for="task in getTasksByStatus('waiting')" 
            :key="task.taskId" 
            class="download-item pb-6 mb-6"
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
                  <i class="fa fa-clock-o text-accent"></i>
                </div>
                <div>
                  <h4 
                    class="font-medium"
                    :class="{
                      'text-white': currentTheme === 'dark',
                      'text-gray-900': currentTheme === 'light'
                    }"
                  >{{ task.fileName }}</h4>
                  <div 
                    class="flex items-center text-xs"
                    :class="{
                      'text-gray-400': currentTheme === 'dark',
                      'text-gray-500': currentTheme === 'light'
                    }"
                  >
                    <span>来源: {{ task.fileName }}</span>
                  </div>
                </div>
              </div>
              <div 
                class="text-sm"
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >
                等待中
              </div>
            </div>
            
            <div class="flex items-center justify-between text-sm">
              <div 
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >
                <i class="fa fa-clock-o"></i>
                <span>等待下载...</span>
              </div>
              <div class="flex space-x-2">
                <button class="text-accent hover:text-accentLight" title="开始" @click="startWaitingTask(task.taskId)">
                  <i class="fa fa-play"></i>
                </button>
                <button 
                  title="取消" 
                  @click="cancelDownload(task.taskId)"
                  :class="{
                    'text-gray-400 hover:text-white': currentTheme === 'dark',
                    'text-gray-500 hover:text-gray-900': currentTheme === 'light'
                  }"
                >
                  <i class="fa fa-times"></i>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- Completed Downloads -->
      <div id="completed-downloads" class="download-content p-6" v-if="activeTab === 'completed'">
        <div id="no-completed-downloads" class="text-center py-10" v-if="getTasksByStatus('completed').length === 0">
          <div 
            class="w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4"
            :class="{
              'bg-gray-700': currentTheme === 'dark',
              'bg-gray-100': currentTheme === 'light'
            }"
          >
            <i class="fa fa-check text-2xl" :class="{
              'text-gray-500': currentTheme === 'dark',
              'text-gray-400': currentTheme === 'light'
            }"></i>
          </div>
          <h3 
            class="text-lg font-semibold mb-2"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >没有已完成的下载</h3>
          <p 
            class="mb-4"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >完成的下载任务将显示在这里</p>
        </div>
        
        <div id="completed-downloads-list" v-else>
          <!-- Completed download items -->
          <div 
            v-for="task in getTasksByStatus('completed')" 
            :key="task.taskId" 
            class="download-item pb-6 mb-6"
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
                  <i class="fa fa-film text-accent"></i>
                </div>
                <div>
                  <h4 
                    class="font-medium"
                    :class="{
                      'text-white': currentTheme === 'dark',
                      'text-gray-900': currentTheme === 'light'
                    }"
                  >{{ task.fileName }}</h4>
                  <div 
                    class="flex items-center text-xs"
                    :class="{
                      'text-gray-400': currentTheme === 'dark',
                      'text-gray-500': currentTheme === 'light'
                    }"
                  >
                    <span>完成于: {{ task.endTime ? new Date(task.endTime).toLocaleString() : '' }}</span>
                  </div>
                </div>
              </div>
              <div 
                class="text-sm"
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >
                {{ formatFileSize(task.totalSize) }}
              </div>
            </div>
            
            <div class="flex items-center justify-between text-sm">
              <div class="text-accent">
                <i class="fa fa-check-circle"></i>
                <span>下载完成</span>
              </div>
              <div class="flex space-x-2">
                <button class="text-accent hover:text-accentLight" title="播放">
                  <i class="fa fa-play"></i>
                </button>
                <button class="text-info hover:text-blue-400" title="转码">
                  <i class="fa fa-exchange"></i>
                </button>
                <button 
                  title="更多"
                  :class="{
                    'text-gray-400 hover:text-white': currentTheme === 'dark',
                    'text-gray-500 hover:text-gray-900': currentTheme === 'light'
                  }"
                >
                  <i class="fa fa-ellipsis-v"></i>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- Paused Downloads -->
      <div id="paused-downloads" class="download-content p-6" v-if="activeTab === 'paused'">
        <!-- Paused download items -->
        <div class="text-center py-10" v-if="getTasksByStatus('paused').length === 0">
          <div 
            class="w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4"
            :class="{
              'bg-gray-700': currentTheme === 'dark',
              'bg-gray-100': currentTheme === 'light'
            }"
          >
            <i class="fa fa-pause text-2xl" :class="{
              'text-gray-500': currentTheme === 'dark',
              'text-gray-400': currentTheme === 'light'
            }"></i>
          </div>
          <h3 
            class="text-lg font-semibold mb-2"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >没有已暂停的下载</h3>
          <p 
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >暂停的下载任务将显示在这里</p>
        </div>
        
        <div id="paused-downloads-list" v-else>
          <div 
            v-for="task in getTasksByStatus('paused')" 
            :key="task.taskId" 
            class="download-item pb-6 mb-6"
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
                  <i class="fa fa-film text-accent"></i>
                </div>
                <div>
                  <h4 
                    class="font-medium"
                    :class="{
                      'text-white': currentTheme === 'dark',
                      'text-gray-900': currentTheme === 'light'
                    }"
                  >{{ task.fileName }}</h4>
                  <div 
                    class="flex items-center text-xs"
                    :class="{
                      'text-gray-400': currentTheme === 'dark',
                      'text-gray-500': currentTheme === 'light'
                    }"
                  >
                    <span>来源: {{ task.fileName }}</span>
                  </div>
                </div>
              </div>
              <div 
                class="text-sm"
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >
                {{ formatFileSize(task.downloaded) }} / {{ formatFileSize(task.totalSize) }}
              </div>
            </div>
            
            <div 
              class="w-full rounded-full h-2.5 mb-2"
              :class="{
                'bg-gray-700': currentTheme === 'dark',
                'bg-gray-200': currentTheme === 'light'
              }"
            >
              <div 
                class="bg-accent h-2.5 rounded-full progress-bar" 
                :style="{ width: `${task.percentage}%` }"
              ></div>
            </div>
            
            <div class="flex items-center justify-between text-sm">
              <div 
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >
                <span>进度: {{ Math.round(task.percentage) }}%</span>
              </div>
              <div class="flex space-x-2">
                <button class="text-accent hover:text-accentLight" title="继续">
                  <i class="fa fa-play"></i>
                </button>
                <button 
                  title="取消"
                  :class="{
                    'text-gray-400 hover:text-white': currentTheme === 'dark',
                    'text-gray-500 hover:text-gray-900': currentTheme === 'light'
                  }"
                >
                  <i class="fa fa-times"></i>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- Cancelled Downloads -->
      <div id="cancelled-downloads" class="download-content p-6" v-if="activeTab === 'cancelled'">
        <!-- Cancelled download items -->
        <div class="text-center py-10" v-if="getTasksByStatus('cancelled').length === 0">
          <div 
            class="w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4"
            :class="{
              'bg-gray-700': currentTheme === 'dark',
              'bg-gray-100': currentTheme === 'light'
            }"
          >
            <i class="fa fa-times text-2xl" :class="{
              'text-gray-500': currentTheme === 'dark',
              'text-gray-400': currentTheme === 'light'
            }"></i>
          </div>
          <h3 
            class="text-lg font-semibold mb-2"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >没有已取消的下载</h3>
          <p 
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >取消的下载任务将显示在这里</p>
        </div>
        
        <div id="cancelled-downloads-list" v-else>
          <div 
            v-for="task in getTasksByStatus('cancelled')" 
            :key="task.taskId" 
            class="download-item pb-6 mb-6"
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
                  <i class="fa fa-film text-accent"></i>
                </div>
                <div>
                  <h4 
                    class="font-medium"
                    :class="{
                      'text-white': currentTheme === 'dark',
                      'text-gray-900': currentTheme === 'light'
                    }"
                  >{{ task.fileName }}</h4>
                  <div 
                    class="flex items-center text-xs"
                    :class="{
                      'text-gray-400': currentTheme === 'dark',
                      'text-gray-500': currentTheme === 'light'
                    }"
                  >
                    <span>来源: {{ task.fileName }}</span>
                  </div>
                </div>
              </div>
              <div 
                class="text-sm"
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >
                {{ formatFileSize(task.downloaded) }} / {{ formatFileSize(task.totalSize) }}
              </div>
            </div>
            
            <div 
              class="w-full rounded-full h-2.5 mb-2"
              :class="{
                'bg-gray-700': currentTheme === 'dark',
                'bg-gray-200': currentTheme === 'light'
              }"
            >
              <div 
                class="bg-accent h-2.5 rounded-full progress-bar" 
                :style="{ width: `${task.percentage}%` }"
              ></div>
            </div>
            
            <div class="flex items-center justify-between text-sm">
              <div class="text-red-500">
                <i class="fa fa-times-circle"></i>
                <span>下载已取消</span>
              </div>
              <div class="flex space-x-2">
                <button class="text-accent hover:text-accentLight" title="重新下载" @click="restartDownload(task)">
                  <i class="fa fa-redo"></i>
                </button>
                <button 
                  title="删除"
                  :class="{
                    'text-gray-400 hover:text-white': currentTheme === 'dark',
                    'text-gray-500 hover:text-gray-900': currentTheme === 'light'
                  }"
                >
                  <i class="fa fa-trash"></i>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
/* Downloads specific styles */
.fade-in {
  animation: fadeIn 0.5s ease-in-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.download-tab.active {
  color: white;
  border-bottom: 2px solid #10b981;
}

.download-item {
  animation: slideIn 0.3s ease-in-out;
}

@keyframes slideIn {
  from { transform: translateX(-20px); opacity: 0; }
  to { transform: translateX(0); opacity: 1; }
}

.btn-primary {
  transition: all 0.2s ease;
}

.btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3);
}
</style>