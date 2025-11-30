<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue';
import type { Ref } from 'vue';
import { GetDownloadStatus, GetDiskSpace, GetTranscodeStatus, GetVideoLibrary } from '../../wailsjs/go/main/App';

// Dashboard data
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

interface TranscodeTask {
  taskId: string;
  inputFile: string;
  outputFile: string;
  status: string;
  progress: number;
  speed: string;
  timeRemaining: string;
  startTime: string;
  endTime: string;
  ffmpegCommand: string;
  pid: number;
  error: string;
  videoCodec: string;
  audioCodec: string;
  resolution: string;
  bitrate: string;
}

interface VideoFile {
  name: string;
  size: number;
  path: string;
  extension: string;
  modTime: string;
}

const downloadTasks = ref<DownloadTask[]>([]);
const transcodeTasks = ref<TranscodeTask[]>([]);
const videoFiles = ref<VideoFile[]>([]);
const isLoading = ref(true);

// Stats data
const totalDownloads = ref(0);
const totalSize = ref(0);
const completedTasks = ref(0);
const waitingTasks = ref(0);
const downloadingTasks = ref(0);
const cancelledTasks = ref(0);

// Transcode stats data
const totalTranscodes = ref(0);
const completedTranscodes = ref(0);
const waitingTranscodes = ref(0);
const transcodingTasks = ref(0);
const failedTranscodes = ref(0);

// Video library stats data
const totalVideos = ref(0);
const totalVideoSize = ref(0);
const videoFormats = ref<Record<string, number>>({});
const largestVideo = ref<VideoFile | null>(null);

// Disk space data
const diskSpace = ref({
  drive: '',
  total: 0,
  available: 0,
  used: 0
});

// Format file size to human readable format
const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

// Get recent downloads (top 3)
const recentDownloads = computed(() => {
  return [...downloadTasks.value]
    .sort((taskA, taskB) => {
      const timeA = taskA.lastUpdate ? new Date(taskA.lastUpdate).getTime() : 0;
      const timeB = taskB.lastUpdate ? new Date(taskB.lastUpdate).getTime() : 0;
      return timeB - timeA;
    })
    .slice(0, 3);
});

// Get download status from backend
const getDownloadStatus = async () => {
  try {
    isLoading.value = true;
    const result = await GetDownloadStatus('');
    const data = JSON.parse(result);
    
    if (data.tasks) {
      downloadTasks.value = data.tasks as DownloadTask[];
      
      // Calculate stats
      totalDownloads.value = downloadTasks.value.length;
      completedTasks.value = downloadTasks.value.filter(task => task.status === 'completed').length;
      waitingTasks.value = downloadTasks.value.filter(task => task.status === 'waiting').length;
      downloadingTasks.value = downloadTasks.value.filter(task => task.status === 'downloading').length;
      cancelledTasks.value = downloadTasks.value.filter(task => task.status === 'cancelled').length;
      
      // Calculate total size of completed downloads
      totalSize.value = downloadTasks.value
        .filter(task => task.status === 'completed')
        .reduce((sum, task) => sum + task.totalSize, 0);
    }
  } catch (error) {
    console.error('Failed to get download status:', error);
  } finally {
    isLoading.value = false;
  }
};

// Get disk space information from backend
const getDiskSpaceInfo = async () => {
  try {
    const result = await GetDiskSpace();
    const data = JSON.parse(result);
    
    if (data.status === 'success') {
      diskSpace.value = {
        drive: data.drive,
        total: data.total,
        available: data.available,
        used: data.used
      };
    }
  } catch (error) {
    console.error('Failed to get disk space information:', error);
  }
};

// Get transcode status from backend
const getTranscodeStatus = async () => {
  try {
    const result = await GetTranscodeStatus('');
    const data = JSON.parse(result);
    
    if (data.tasks) {
      transcodeTasks.value = data.tasks as TranscodeTask[];
      
      // Calculate transcode stats
      totalTranscodes.value = transcodeTasks.value.length;
      completedTranscodes.value = transcodeTasks.value.filter(task => task.status === 'completed').length;
      waitingTranscodes.value = transcodeTasks.value.filter(task => task.status === 'waiting').length;
      transcodingTasks.value = transcodeTasks.value.filter(task => task.status === 'transcoding').length;
      failedTranscodes.value = transcodeTasks.value.filter(task => task.status === 'failed').length;
    }
  } catch (error) {
    console.error('Failed to get transcode status:', error);
  }
};

// Get video library information from backend
const getVideoLibraryInfo = async () => {
  try {
    const result = await GetVideoLibrary();
    const data = JSON.parse(result);
    
    if (data.status === 'success' && data.videoFiles) {
      videoFiles.value = data.videoFiles as VideoFile[];
      
      // Calculate video library stats
      totalVideos.value = videoFiles.value.length;
      
      // Calculate total video size
      totalVideoSize.value = videoFiles.value.reduce((sum, file) => sum + file.size, 0);
      
      // Calculate video formats distribution
      const formats: Record<string, number> = {};
      videoFiles.value.forEach(file => {
        const ext = file.extension.toLowerCase();
        formats[ext] = (formats[ext] || 0) + 1;
      });
      videoFormats.value = formats;
      
      // Find largest video
      if (videoFiles.value.length > 0) {
        largestVideo.value = [...videoFiles.value].sort((a, b) => b.size - a.size)[0];
      }
    }
  } catch (error) {
    console.error('Failed to get video library information:', error);
  }
};

// Theme management - using global theme from App.vue
const currentTheme = inject('currentTheme') as Ref<string>;
const updateTheme = inject('updateTheme') as (theme: string) => void;

// Image error handling with fallback
const imageUrls = [
  'https://gitee.com/bmcbdt/SeedParser/raw/master/images/zfb.jpg'
];

let currentImageIndex = 0;

const handleImageError = (event: Event) => {
  const img = event.target as HTMLImageElement;
  
  // Try next image URL
  currentImageIndex++;
  if (currentImageIndex < imageUrls.length) {
    img.src = imageUrls[currentImageIndex];
  } else {
    // All images failed, show placeholder
    img.style.display = 'none';
    const parent = img.parentElement;
    if (parent) {
      parent.innerHTML = '<i class="fa fa-qrcode text-4xl text-white/80"></i>';
    }
  }
};

// Lifecycle hooks
onMounted(() => {
  getDownloadStatus();
  getDiskSpaceInfo();
  getTranscodeStatus();
  getVideoLibraryInfo();
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
      >仪表盘</h2>
      <p 
        :class="{
          'text-gray-400': currentTheme === 'dark',
          'text-gray-500': currentTheme === 'light'
        }"
      >欢迎使用 VideoTorrent，一站式视频下载与管理工具</p>
    </div>
    
    <!-- Stats Cards -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
      <div 
        class="rounded-lg p-6 shadow-lg hover-scale transition-all duration-300"
        :class="{
          'bg-secondary': currentTheme === 'dark',
          'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
        }"
      >
        <div class="flex items-center justify-between mb-4">
          <h3 
            class="text-lg font-semibold"
            :class="{
              'text-gray-300': currentTheme === 'dark',
              'text-gray-700': currentTheme === 'light'
            }"
          >总下载量</h3>
          <span class="text-accent text-xl">
            <i class="fa fa-download"></i>
          </span>
        </div>
        <div class="flex items-end">
          <span 
            class="text-3xl font-bold"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >{{ totalDownloads }}</span>
          <span 
            class="ml-2 mb-1"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >个任务</span>
        </div>
        <div class="mt-2 text-sm">
          <span 
            class="text-accent"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >
            <span class="text-accent"><i class="fa fa-check-circle"></i> {{ completedTasks }} 已完成</span>
            <span class="ml-2">{{ waitingTasks }} 等待中</span>
          </span>
        </div>
      </div>
      
      <div 
        class="rounded-lg p-6 shadow-lg hover-scale transition-all duration-300"
        :class="{
          'bg-secondary': currentTheme === 'dark',
          'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
        }"
      >
        <div class="flex items-center justify-between mb-4">
          <h3 
            class="text-lg font-semibold"
            :class="{
              'text-gray-300': currentTheme === 'dark',
              'text-gray-700': currentTheme === 'light'
            }"
          >已用空间</h3>
          <span class="text-info text-xl">
            <i class="fa fa-hdd-o"></i>
          </span>
        </div>
        <div class="flex items-end">
          <span 
            class="text-3xl font-bold"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >{{ formatFileSize(diskSpace.used) }}</span>
          <span 
            class="ml-2 mb-1"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          ></span>
        </div>
        <div class="mt-2 text-sm">
          <div class="w-full rounded-full h-1.5" :class="{
            'bg-gray-700': currentTheme === 'dark',
            'bg-gray-200': currentTheme === 'light'
          }">
            <div class="bg-info h-1.5 rounded-full progress-bar" :style="{ width: `${Math.min((diskSpace.used / diskSpace.total) * 100, 100)}%` }"></div>
          </div>
          <div class="flex justify-between mt-1" :class="{
            'text-gray-400': currentTheme === 'dark',
            'text-gray-500': currentTheme === 'light'
          }">
            <span>{{ formatFileSize(diskSpace.used) }} / {{ formatFileSize(diskSpace.total) }}</span>
            <span>{{ Math.min(Math.round((diskSpace.used / diskSpace.total) * 100), 100) }}%</span>
          </div>
        </div>
      </div>
      
      <div 
        class="rounded-lg p-6 shadow-lg hover-scale transition-all duration-300"
        :class="{
          'bg-secondary': currentTheme === 'dark',
          'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
        }"
      >
        <div class="flex items-center justify-between mb-4">
          <h3 
            class="text-lg font-semibold"
            :class="{
              'text-gray-300': currentTheme === 'dark',
              'text-gray-700': currentTheme === 'light'
            }"
          >等待中的任务</h3>
          <span class="text-warning text-xl">
            <i class="fa fa-clock-o"></i>
          </span>
        </div>
        <div class="flex items-end">
          <span 
            class="text-3xl font-bold"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >{{ waitingTasks }}</span>
          <span 
            class="ml-2 mb-1"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >个任务</span>
        </div>
        <div class="mt-2 text-sm">
          <span 
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >
            <span class="text-warning"><i class="fa fa-clock-o"></i> 等待下载</span>
            <span class="ml-2">{{ downloadingTasks }} 正在下载</span>
          </span>
        </div>
      </div>
      
      <div 
        class="rounded-lg p-6 shadow-lg hover-scale transition-all duration-300"
        :class="{
          'bg-secondary': currentTheme === 'dark',
          'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
        }"
      >
        <div class="flex items-center justify-between mb-4">
          <h3 
            class="text-lg font-semibold"
            :class="{
              'text-gray-300': currentTheme === 'dark',
              'text-gray-700': currentTheme === 'light'
            }"
          >已完成任务</h3>
          <span class="text-success text-xl">
            <i class="fa fa-check-circle"></i>
          </span>
        </div>
        <div class="flex items-end">
          <span 
            class="text-3xl font-bold"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >{{ completedTasks }}</span>
          <span 
            class="ml-2 mb-1"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >个任务</span>
        </div>
        <div class="mt-2 text-sm">
          <span 
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >
            <span class="text-success"><i class="fa fa-check"></i> 已完成</span>
            <span class="ml-2">{{ cancelledTasks }} 已取消</span>
          </span>
        </div>
      </div>
      
      <div 
        class="rounded-lg p-6 shadow-lg hover-scale transition-all duration-300"
        :class="{
          'bg-secondary': currentTheme === 'dark',
          'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
        }"
      >
        <div class="flex items-center justify-between mb-4">
          <h3 
            class="text-lg font-semibold"
            :class="{
              'text-gray-300': currentTheme === 'dark',
              'text-gray-700': currentTheme === 'light'
            }"
          >视频转码</h3>
          <span class="text-purple-500 text-xl">
            <i class="fa fa-exchange"></i>
          </span>
        </div>
        <div class="flex items-end">
          <span 
            class="text-3xl font-bold"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >{{ totalTranscodes }}</span>
          <span 
            class="ml-2 mb-1"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >个任务</span>
        </div>
        <div class="mt-2 text-sm">
          <span 
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >
            <span class="text-accent"><i class="fa fa-check-circle"></i> {{ completedTranscodes }} 已完成</span>
            <span class="ml-2">{{ transcodingTasks }} 转码中</span>
          </span>
        </div>
      </div>
      
      <div 
        class="rounded-lg p-6 shadow-lg hover-scale transition-all duration-300"
        :class="{
          'bg-secondary': currentTheme === 'dark',
          'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
        }"
      >
        <div class="flex items-center justify-between mb-4">
          <h3 
            class="text-lg font-semibold"
            :class="{
              'text-gray-300': currentTheme === 'dark',
              'text-gray-700': currentTheme === 'light'
            }"
          >视频库</h3>
          <span class="text-blue-500 text-xl">
            <i class="fa fa-film"></i>
          </span>
        </div>
        <div class="flex items-end">
          <span 
            class="text-3xl font-bold"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >{{ totalVideos }}</span>
          <span 
            class="ml-2 mb-1"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >个视频</span>
        </div>
        <div class="mt-2 text-sm">
          <span 
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >
            <span class="text-blue-500"><i class="fa fa-hdd-o"></i> {{ formatFileSize(totalVideoSize) }}</span>
            <span class="ml-2">{{ Object.keys(videoFormats).length }} 种格式</span>
          </span>
        </div>
      </div>
      
      <div 
        class="rounded-lg p-6 shadow-lg hover-scale transition-all duration-300"
        :class="{
          'bg-secondary': currentTheme === 'dark',
          'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
        }"
      >
        <div class="flex items-center justify-between mb-4">
          <h3 
            class="text-lg font-semibold"
            :class="{
              'text-gray-300': currentTheme === 'dark',
              'text-gray-700': currentTheme === 'light'
            }"
          >主题设置</h3>
          <span class="text-purple-500 text-xl">
            <i class="fa fa-paint-brush"></i>
          </span>
        </div>
        <div class="space-y-4">
          <div class="flex items-center justify-between">
            <span 
              :class="{
                'text-gray-300': currentTheme === 'dark',
                'text-gray-700': currentTheme === 'light'
              }"
            >当前主题</span>
            <span 
              class="font-medium capitalize"
              :class="{
                'text-white': currentTheme === 'dark',
                'text-gray-900': currentTheme === 'light'
              }"
            >{{ currentTheme }}</span>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <button 
              class="py-2 px-4 rounded-lg font-medium transition-all hover:scale-105"
              :class="{
                'bg-accent text-white': currentTheme === 'dark',
                'bg-gray-200 text-gray-700 hover:bg-gray-300': currentTheme !== 'dark'
              }"
              @click="updateTheme('dark')"
            >
              <div class="flex items-center justify-center">
                <i class="fa fa-moon-o mr-2"></i>
                深色模式
              </div>
            </button>
            <button 
              class="py-2 px-4 rounded-lg font-medium transition-all hover:scale-105"
              :class="{
                'bg-accent text-white': currentTheme === 'light',
                'bg-gray-200 text-gray-700 hover:bg-gray-300': currentTheme !== 'light'
              }"
              @click="updateTheme('light')"
            >
              <div class="flex items-center justify-center">
                <i class="fa fa-sun-o mr-2"></i>
                浅色模式
              </div>
            </button>
          </div>
        </div>
      </div>
    </div>
    
    <!-- Video Library Stats -->
    <div 
      class="rounded-lg p-6 shadow-lg mb-8 transition-all duration-300"
      :class="{
        'bg-secondary': currentTheme === 'dark',
        'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
      }"
    >
      <div class="flex items-center justify-between mb-6">
        <h3 
          class="text-xl font-semibold"
          :class="{
            'text-white': currentTheme === 'dark',
            'text-gray-900': currentTheme === 'light'
          }"
        >视频库统计</h3>
        <router-link to="/library" class="text-accent hover:text-accentLight text-sm flex items-center">
          查看全部
          <i class="fa fa-arrow-right ml-1"></i>
        </router-link>
      </div>
      
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div 
          class="rounded-lg p-4"
          :class="{
            'bg-gray-800': currentTheme === 'dark',
            'bg-gray-50 border border-gray-200': currentTheme === 'light'
          }"
        >
          <h4 
            class="text-sm mb-2"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >视频格式分布</h4>
          <div class="space-y-2">
            <div v-for="(count, format) in videoFormats" :key="format" class="flex items-center justify-between">
              <span 
                class="text-sm"
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >{{ format.toUpperCase() }}</span>
              <span 
                class="font-medium"
                :class="{
                  'text-white': currentTheme === 'dark',
                  'text-gray-900': currentTheme === 'light'
                }"
              >{{ count }}</span>
            </div>
            <div v-if="Object.keys(videoFormats).length === 0" class="text-center text-sm">
              <span 
                :class="{
                  'text-gray-500': currentTheme === 'dark',
                  'text-gray-400': currentTheme === 'light'
                }"
              >暂无视频文件</span>
            </div>
          </div>
        </div>
        
        <div 
          class="rounded-lg p-4"
          :class="{
            'bg-gray-800': currentTheme === 'dark',
            'bg-gray-50 border border-gray-200': currentTheme === 'light'
          }"
        >
          <h4 
            class="text-sm mb-2"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >视频存储</h4>
          <div class="space-y-4">
            <div>
              <div class="flex items-center justify-between mb-1">
                <span 
                  class="text-sm"
                  :class="{
                    'text-gray-300': currentTheme === 'dark',
                    'text-gray-700': currentTheme === 'light'
                  }"
                >总视频大小</span>
                <span 
                  class="font-medium"
                  :class="{
                    'text-white': currentTheme === 'dark',
                    'text-gray-900': currentTheme === 'light'
                  }"
                >{{ formatFileSize(totalVideoSize) }}</span>
              </div>
              <div class="w-full rounded-full h-1.5" :class="{
                'bg-gray-700': currentTheme === 'dark',
                'bg-gray-200': currentTheme === 'light'
              }">
                <div class="bg-blue-500 h-1.5 rounded-full" :style="{ width: `${Math.min((totalVideoSize / diskSpace.total) * 100, 100)}%` }"></div>
              </div>
            </div>
            <div v-if="largestVideo" class="text-sm">
              <div 
                class="mb-1"
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >最大视频</div>
              <div 
                :class="{
                  'text-white truncate': currentTheme === 'dark',
                  'text-gray-900 truncate': currentTheme === 'light'
                }"
              >{{ largestVideo.name }}</div>
              <div 
                :class="{
                  'text-gray-500 mt-1': currentTheme === 'dark',
                  'text-gray-400 mt-1': currentTheme === 'light'
                }"
              >{{ formatFileSize(largestVideo.size) }}</div>
            </div>
            <div v-else class="text-center text-sm">
              <span 
                :class="{
                  'text-gray-500': currentTheme === 'dark',
                  'text-gray-400': currentTheme === 'light'
                }"
              >暂无视频文件</span>
            </div>
          </div>
        </div>
        
        <div 
          class="rounded-lg p-4"
          :class="{
            'bg-gray-800': currentTheme === 'dark',
            'bg-gray-50 border border-gray-200': currentTheme === 'light'
          }"
        >
          <h4 
            class="text-sm mb-2"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >视频统计</h4>
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <span 
                class="text-sm"
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >总视频数</span>
              <span 
                class="font-medium"
                :class="{
                  'text-white': currentTheme === 'dark',
                  'text-gray-900': currentTheme === 'light'
                }"
              >{{ totalVideos }}</span>
            </div>
            <div class="flex items-center justify-between">
              <span 
                class="text-sm"
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >平均视频大小</span>
              <span 
                class="font-medium"
                :class="{
                  'text-white': currentTheme === 'dark',
                  'text-gray-900': currentTheme === 'light'
                }"
              >{{ totalVideos > 0 ? formatFileSize(totalVideoSize / totalVideos) : '0 B' }}</span>
            </div>
            <div class="flex items-center justify-between">
              <span 
                class="text-sm"
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >视频格式数</span>
              <span 
                class="font-medium"
                :class="{
                  'text-white': currentTheme === 'dark',
                  'text-gray-900': currentTheme === 'light'
                }"
              >{{ Object.keys(videoFormats).length }}</span>
            </div>
            <div class="flex items-center justify-between">
              <span 
                class="text-sm"
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >视频占用空间</span>
              <span 
                class="font-medium"
                :class="{
                  'text-white': currentTheme === 'dark',
                  'text-gray-900': currentTheme === 'light'
                }"
              >{{ diskSpace.total > 0 ? `${Math.round((totalVideoSize / diskSpace.total) * 100)}%` : '0%' }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
    
    <!-- Recent Downloads -->
    <div 
      class="rounded-lg p-6 shadow-lg mb-8 transition-all duration-300"
      :class="{
        'bg-secondary': currentTheme === 'dark',
        'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
      }"
    >
      <div class="flex items-center justify-between mb-6">
        <h3 
          class="text-xl font-semibold"
          :class="{
            'text-white': currentTheme === 'dark',
            'text-gray-900': currentTheme === 'light'
          }"
        >最近下载</h3>
        <router-link to="/downloads" class="text-accent hover:text-accentLight text-sm flex items-center">
          查看全部
          <i class="fa fa-arrow-right ml-1"></i>
        </router-link>
      </div>
      
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
              >状态</th>
              <th 
                class="pb-3 font-medium"
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >操作</th>
            </tr>
          </thead>
          <tbody>
            <tr 
              v-if="isLoading" 
              :class="{
                'border-b border-gray-700': currentTheme === 'dark',
                'border-b border-gray-200': currentTheme === 'light'
              }"
            >
              <td 
                class="py-4 text-center"
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
                colspan="4">加载中...</td>
            </tr>
            <tr 
              v-else-if="downloadTasks.length === 0" 
              :class="{
                'border-b border-gray-700': currentTheme === 'dark',
                'border-b border-gray-200': currentTheme === 'light'
              }"
            >
              <td 
                class="py-4 text-center"
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
                colspan="4">暂无下载记录</td>
            </tr>
            <tr 
              v-else 
              v-for="task in recentDownloads" 
              :key="task.taskId" 
              :class="{
                'border-b border-gray-700 hover:bg-gray-800': currentTheme === 'dark',
                'border-b border-gray-200 hover:bg-gray-50': currentTheme === 'light'
              }"
            >
              <td class="py-4">
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
                  <span 
                    :class="{
                      'text-gray-300': currentTheme === 'dark',
                      'text-gray-700': currentTheme === 'light'
                    }"
                  >{{ task.fileName }}</span>
                </div>
              </td>
              <td 
                class="py-4"
                :class="{
                  'text-gray-400': currentTheme === 'dark',
                  'text-gray-500': currentTheme === 'light'
                }"
              >{{ formatFileSize(task.totalSize) }}</td>
              <td class="py-4">
                <span 
                  class="px-2 py-1 rounded-full text-xs font-medium"
                  :class="{
                    'bg-green-900 text-green-300': task.status === 'completed',
                    'bg-yellow-900 text-yellow-300': task.status === 'waiting',
                    'bg-blue-900 text-blue-300': task.status === 'downloading',
                    'bg-red-900 text-red-300': task.status === 'cancelled',
                    'bg-green-100 text-green-800': currentTheme === 'light' && task.status === 'completed',
                    'bg-yellow-100 text-yellow-800': currentTheme === 'light' && task.status === 'waiting',
                    'bg-blue-100 text-blue-800': currentTheme === 'light' && task.status === 'downloading',
                    'bg-red-100 text-red-800': currentTheme === 'light' && task.status === 'cancelled'
                  }"
                >
                  {{ task.status === 'completed' ? '已完成' : 
                     task.status === 'waiting' ? '等待中' : 
                     task.status === 'downloading' ? '下载中' : '已取消' }}
                </span>
              </td>
              <td class="py-4">
                <div class="flex space-x-2">
                  <button 
                    class="text-accent hover:text-accentLight" 
                    title="播放"
                    v-if="task.status === 'completed'"
                  >
                    <i class="fa fa-play"></i>
                  </button>
                  <button 
                    class="text-info hover:text-blue-400" 
                    title="转码"
                    v-if="task.status === 'completed'"
                  >
                    <i class="fa fa-exchange"></i>
                  </button>
                  <button 
                    class="hover:text-white" 
                    title="更多"
                    :class="{
                      'text-gray-400': currentTheme === 'dark',
                      'text-gray-500': currentTheme === 'light'
                    }"
                  >
                    <i class="fa fa-ellipsis-v"></i>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    
    <!-- Quick Actions -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
      <div 
        class="rounded-lg p-6 shadow-lg transition-all duration-300"
        :class="{
          'bg-secondary': currentTheme === 'dark',
          'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
        }"
      >
        <h3 
          class="text-xl font-semibold mb-4"
          :class="{
            'text-white': currentTheme === 'dark',
            'text-gray-900': currentTheme === 'light'
          }"
        >快速操作</h3>
        <div class="grid grid-cols-2 gap-4">
          <router-link to="/torrent" 
            class="btn-primary bg-accent hover:bg-accentDark text-white py-3 px-4 rounded-lg flex items-center justify-center transition-all hover:scale-105"
          >
            <i class="fa fa-magnet mr-2"></i>
            <span>解析种子</span>
          </router-link>
          <router-link to="/transcode" 
            class="btn-secondary py-3 px-4 rounded-lg flex items-center justify-center transition-all hover:scale-105"
            :class="{
              'bg-info hover:bg-blue-600 text-white': currentTheme === 'dark',
              'bg-blue-500 hover:bg-blue-600 text-white': currentTheme === 'light'
            }"
          >
            <i class="fa fa-exchange mr-2"></i>
            <span>视频转码</span>
          </router-link>
          <router-link to="/library" 
            class="btn-secondary py-3 px-4 rounded-lg flex items-center justify-center transition-all hover:scale-105"
            :class="{
              'bg-secondary border border-gray-600 hover:bg-gray-700 text-white': currentTheme === 'dark',
              'bg-gray-100 border border-gray-200 hover:bg-gray-200 text-gray-900': currentTheme === 'light'
            }"
          >
            <i class="fa fa-film mr-2"></i>
            <span>浏览视频</span>
          </router-link>
          <button 
            class="btn-secondary py-3 px-4 rounded-lg flex items-center justify-center transition-all hover:scale-105"
            :class="{
              'bg-secondary border border-gray-600 hover:bg-gray-700 text-white': currentTheme === 'dark',
              'bg-gray-100 border border-gray-200 hover:bg-gray-200 text-gray-900': currentTheme === 'light'
            }"
          >
            <i class="fa fa-cog mr-2"></i>
            <span>设置</span>
          </button>
        </div>
      </div>
      
      <div 
        class="rounded-lg p-6 shadow-lg transition-all duration-300"
        :class="{
          'bg-secondary': currentTheme === 'dark',
          'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
        }"
      >
        <h3 
          class="text-xl font-semibold mb-4"
          :class="{
            'text-white': currentTheme === 'dark',
            'text-gray-900': currentTheme === 'light'
          }"
        >存储空间</h3>
        <div class="mb-4">
          <div class="w-full rounded-full h-2.5" :class="{
            'bg-gray-700': currentTheme === 'dark',
            'bg-gray-200': currentTheme === 'light'
          }">
            <div class="bg-info h-2.5 rounded-full progress-bar" :style="{ width: `${Math.min((diskSpace.used / diskSpace.total) * 100, 100)}%` }"></div>
          </div>
          <div class="flex justify-between mt-1 text-sm">
            <span 
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            >已使用: {{ formatFileSize(diskSpace.used) }}</span>
            <span 
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            >总空间: {{ formatFileSize(diskSpace.total) }}</span>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4 text-sm">
          <div>
            <div class="flex items-center mb-1">
              <span class="w-3 h-3 bg-accent rounded-full mr-2"></span>
              <span 
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >已下载文件</span>
            </div>
            <div 
              class="ml-5"
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            >{{ formatFileSize(totalSize) }} ({{ Math.round((totalSize / diskSpace.used) * 100) }}%)</div>
          </div>
          <div>
            <div class="flex items-center mb-1">
              <span class="w-3 h-3 bg-warning rounded-full mr-2"></span>
              <span 
                :class="{
                  'text-gray-300': currentTheme === 'dark',
                  'text-gray-700': currentTheme === 'light'
                }"
              >其他文件</span>
            </div>
            <div 
              class="ml-5"
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            >{{ formatFileSize(diskSpace.used - totalSize) }} ({{ Math.round(((diskSpace.used - totalSize) / diskSpace.used) * 100) }}%)</div>
          </div>
        </div>
        <button 
          class="mt-4 text-sm flex items-center"
          :class="{
            'text-info hover:text-blue-400': currentTheme === 'dark',
            'text-blue-600 hover:text-blue-800': currentTheme === 'light'
          }"
        >
          管理存储空间
          <i class="fa fa-arrow-right ml-1"></i>
        </button>
      </div>
    </div>
    
    <!-- 个人介绍模块 -->
    <div 
      class="rounded-lg p-6 shadow-lg transition-all duration-300"
      :class="{
        'bg-secondary': currentTheme === 'dark',
        'bg-white border border-gray-200 shadow-md': currentTheme === 'light'
      }"
    >
      <h3 
        class="text-xl font-semibold mb-4"
        :class="{
          'text-white': currentTheme === 'dark',
          'text-gray-900': currentTheme === 'light'
        }"
      >关于作者</h3>
      <div class="flex flex-col md:flex-row items-center">
        <div class="w-24 h-24 rounded-full flex items-center justify-center mr-6 mb-4 md:mb-0" :class="{
          'bg-gray-700': currentTheme === 'dark',
          'bg-gray-100': currentTheme === 'light'
        }">
          <i class="fa fa-user text-4xl text-accent"></i>
        </div>
        <div class="flex-1">
          <h4 
            class="text-lg font-medium mb-2"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >开源作者介绍</h4>
          <p 
            class="mb-4"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-600': currentTheme === 'light'
            }"
          >这是一个开源视频下载与管理工具，由热爱技术的开发者精心打造。项目采用 Wails 框架（Go 后端 + Vue 前端）开发，支持种子解析、视频转码、库管理等功能。</p>
          <div class="flex space-x-4">
            <a href="https://github.com/kamisamadasikede/SeedParser" target="_blank" 
              class="text-info hover:text-blue-400 flex items-center"
              :class="{
                'text-info hover:text-blue-400': currentTheme === 'dark',
                'text-blue-600 hover:text-blue-800': currentTheme === 'light'
              }"
            >
              <i class="fa fa-github mr-1"></i> GitHub
            </a>
            <a href="https://gitee.com/bmcbdt/SeedParser" target="_blank" 
              class="text-info hover:text-blue-400 flex items-center"
              :class="{
                'text-info hover:text-blue-400': currentTheme === 'dark',
                'text-blue-600 hover:text-blue-800': currentTheme === 'light'
              }"
            >
              <i class="fa fa-gitlab mr-1"></i> Gitee
            </a>
            <a href="mailto:3223694732@qq.com" 
              class="text-info hover:text-blue-400 flex items-center"
              :class="{
                'text-info hover:text-blue-400': currentTheme === 'dark',
                'text-blue-600 hover:text-blue-800': currentTheme === 'light'
              }"
            >
              <i class="fa fa-envelope mr-1"></i> 联系我
            </a>
          </div>
        </div>
      </div>
    </div>

    <!-- 支持作者模块 -->
    <div 
      class="rounded-lg p-6 shadow-lg transition-all duration-300"
      :class="{
        'bg-gradient-to-br from-orange-500 to-pink-500': currentTheme === 'dark',
        'bg-gradient-to-br from-orange-400 to-pink-400': currentTheme === 'light'
      }"
    >
      <div class="text-center text-white">
        <div class="mb-4">
          <i class="fa fa-coffee text-4xl mb-2"></i>
          <h3 class="text-2xl font-bold mb-2">如果好用请作者一杯奶茶喝 ☕</h3>
          <p class="text-lg opacity-90">您的支持是作者继续开发的最大动力！</p>
        </div>
        
        <div class="flex flex-col md:flex-row items-center justify-center gap-8 mt-6">
          <!-- 支付宝收款码区域 -->
          <div class="bg-white/10 backdrop-blur-sm rounded-lg p-6 text-center">
            <div class="w-32 h-48 bg-white/20 rounded-lg flex items-center justify-center mb-3 mx-auto overflow-hidden">
              <!-- 支付宝收款码图片 -->
              <img 
                src="https://github.com/kamisamadasikede/SeedParser/raw/master/images/zfb.jpg" 
                alt="支付宝收款码" 
                class="w-full h-full object-cover rounded-lg"
                @error="handleImageError"
                loading="lazy"
              >
            </div>
            <p class="text-sm font-medium mb-2">支付宝扫码打赏</p>
            <p class="text-xs opacity-75">请作者喝杯奶茶吧</p>
          </div>
          
          <!-- 感谢文字 -->
          <div class="text-left">
            <h4 class="text-lg font-semibold mb-3">感谢您的支持 🙏</h4>
            <ul class="space-y-2 text-sm opacity-90">
              <li class="flex items-center">
                <i class="fa fa-heart text-red-300 mr-2"></i>
                您的支持将用于服务器维护
              </li>
              <li class="flex items-center">
                <i class="fa fa-heart text-red-300 mr-2"></i>
                帮助开发更多实用功能
              </li>
              <li class="flex items-center">
                <i class="fa fa-heart text-red-300 mr-2"></i>
                提升软件稳定性和性能
              </li>
              <li class="flex items-center">
                <i class="fa fa-heart text-red-300 mr-2"></i>
                持续优化用户体验
              </li>
            </ul>
          </div>
        </div>
        
        <div class="mt-6 pt-4 border-t border-white/20">
          <p class="text-sm opacity-75">
            💝 每一份支持都是对开源精神的认可，让我们一起打造更好的工具！
          </p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
/* Dashboard specific styles */
.fade-in {
  animation: fadeIn 0.5s ease-in-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.btn-primary {
  transition: all 0.2s ease;
}

.btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3);
}

.btn-secondary {
  transition: all 0.2s ease;
}

.btn-secondary:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}
</style>