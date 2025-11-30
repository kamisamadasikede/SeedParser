<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue';
import { GetVideoLibrary, ServeVideoFile } from '../../wailsjs/go/main/App';

// Theme management - using global theme from App.vue
const currentTheme = inject('currentTheme');
const updateTheme = inject('updateTheme');

// Video library data
interface VideoFile {
  name: string;
  size: number;
  path: string;
  extension: string;
  modTime: string;
}

const videoFiles = ref<VideoFile[]>([]);
const isLoading = ref(true);
const searchQuery = ref('');
const selectedFormat = ref('all');

// Video player data
const showVideoPlayer = ref(false);
const currentVideo = ref<VideoFile | null>(null);
const videoThumbnails = ref<Record<string, string>>({});
const generatingThumbnails = ref<Record<string, boolean>>({});
const downloadingVideos = ref<Record<string, boolean>>({});

// Thumbnail generation queue
const thumbnailQueue = ref<VideoFile[]>([]);
const maxConcurrentThumbnails = 4; // 增加并行处理数量，提高生成速度
const isProcessingQueue = ref(false);

// Format file size to human readable format
const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

// Handle image load error
const handleImageError = (event: Event) => {
  const target = event.target as HTMLImageElement;
  if (target) {
    target.style.display = 'none';
  }
};

// Process thumbnail generation queue
const processThumbnailQueue = async () => {
  if (isProcessingQueue.value) return;
  
  isProcessingQueue.value = true;
  
  try {
    while (thumbnailQueue.value.length > 0) {
      // Check if we're already generating max thumbnails
      const currentGenerating = (Object.values(generatingThumbnails.value) as boolean[]).filter(Boolean).length;
      if (currentGenerating >= maxConcurrentThumbnails) {
        // Wait for a thumbnail to finish
        await new Promise(resolve => setTimeout(resolve, 100));
        continue;
      }
      
      // Get next video from queue
      const video = thumbnailQueue.value.shift();
      if (!video) break;
      
      // Skip if already generated or generating
      if (videoThumbnails.value[video.name] || generatingThumbnails.value[video.name]) {
        continue;
      }
      
      // Generate thumbnail
      generatingThumbnails.value[video.name] = true;
      
      try {
        const thumbnail = await generateThumbnail(video.path, video.name);
        if (thumbnail) {
          videoThumbnails.value[video.name] = thumbnail;
        }
      } catch (error) {
        console.error(`生成视频 ${video.name} 缩略图失败:`, error);
      } finally {
        generatingThumbnails.value[video.name] = false;
      }
    }
  } finally {
    isProcessingQueue.value = false;
  }
};

// Get video library from backend
const getVideoLibrary = async () => {
  try {
    isLoading.value = true;
    const result = await GetVideoLibrary();
    const data = JSON.parse(result);
    
    if (data.status === 'success' && data.videoFiles) {
      videoFiles.value = data.videoFiles as VideoFile[];
      
      // Clear existing queue and add all videos to queue
      thumbnailQueue.value = [...videoFiles.value];
      
      // Start processing queue
      processThumbnailQueue();
    }
  } catch (error) {
    console.error('Failed to get video library:', error);
  } finally {
    isLoading.value = false;
  }
};

// Generate video thumbnail
const generateThumbnail = async (videoPath: string, videoName: string): Promise<string> => {
  return new Promise((resolve) => {
    const video = document.createElement('video');
    const canvas = document.createElement('canvas');
    const context = canvas.getContext('2d');
    
    if (!context) {
      resolve('');
      return;
    }
    
    // 优化：使用合适的缩略图尺寸，平衡速度和质量
    canvas.width = 320;
    canvas.height = 180;
    
    // 优化：降低预加载级别，只加载元数据
    video.preload = 'metadata';
    video.muted = true;
    video.playsInline = true;
    video.crossOrigin = 'anonymous';
    
    // 优化：直接设置视频源，跳过HEAD请求
    video.src = `/downloads/${videoName}`;
    
    // 优化：加载元数据后立即跳转到目标帧
    video.onloadedmetadata = () => {
      // 计算第20帧的时间（假设30fps）
      const targetTime = Math.min(0.67, video.duration - 0.1); // 20/30 = 0.67秒
      video.currentTime = targetTime;
    };
    
    // 优化：跳转到指定帧后立即绘制，不等待完全加载
    video.onseeked = () => {
      try {
        // 绘制视频帧
        context.drawImage(video, 0, 0, canvas.width, canvas.height);
        
        // 优化：使用合适的JPEG质量，平衡速度和质量
        const thumbnail = canvas.toDataURL('image/jpeg', 0.7);
        
        // 优化：立即停止视频加载，释放资源
        video.pause();
        video.src = '';
        video.load();
        
        resolve(thumbnail);
      } catch {
        resolve('');
      }
    };
    
    // 优化：处理加载错误
    video.onerror = () => {
      resolve('');
    };
    
    // 优化：进一步缩短超时时间
    setTimeout(() => {
      // 清理资源
      video.pause();
      video.src = '';
      video.load();
      resolve('');
    }, 2000);
    
    // 开始加载
    video.load();
  });
};

// Play video
const playVideo = async (video: VideoFile) => {
  try {
    // 调用后端API获取安全的视频文件访问
    const result = await ServeVideoFile(video.name);
    const data = JSON.parse(result);
    
    if (data.status === 'success') {
      currentVideo.value = video;
      showVideoPlayer.value = true;
    } else {
      console.error('Failed to serve video file:', data);
    }
  } catch (error) {
    console.error('Error serving video file:', error);
  }
};

// Download video
const downloadVideo = async (video: VideoFile) => {
  try {
    downloadingVideos.value[video.name] = true;
    
    const downloadUrl = `/downloads/${video.name}`;
    
    // 方法1: 使用fetch API获取文件并创建blob URL（更现代的方法）
    try {
      const response = await fetch(downloadUrl);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      
      const blob = await response.blob();
      const blobUrl = window.URL.createObjectURL(blob);
      
      // 创建下载链接
      const link = document.createElement('a');
      link.href = blobUrl;
      link.download = video.name;
      link.style.display = 'none';
      
      document.body.appendChild(link);
      link.click();
      
      // 清理
      setTimeout(() => {
        window.URL.revokeObjectURL(blobUrl);
        document.body.removeChild(link);
      }, 100);
      
    } catch (fetchError) {
      console.warn('Fetch API下载失败，回退到传统方法:', fetchError);
      
      // 方法2: 传统方法（回退方案）
      const link = document.createElement('a');
      link.href = downloadUrl;
      link.download = video.name;
      link.style.display = 'none';
      
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    }
    
    console.log(`开始下载视频: ${video.name}`);
    
    // 下载完成提示
    setTimeout(() => {
      downloadingVideos.value[video.name] = false;
      // 可以在这里添加下载完成提示
    }, 3000);
    
  } catch (error) {
    console.error('下载视频失败:', error);
    alert('下载视频失败，请重试');
    downloadingVideos.value[video.name] = false;
  }
};

// Close video player
const closeVideoPlayer = () => {
  showVideoPlayer.value = false;
  currentVideo.value = null;
};

// Filter videos based on search query and selected format
const filteredVideos = computed(() => {
  return videoFiles.value.filter(video => {
    // Filter by search query
    const matchesSearch = video.name.toLowerCase().includes(searchQuery.value.toLowerCase());
    
    // Filter by format
    const matchesFormat = selectedFormat.value === 'all' || video.extension === selectedFormat.value;
    
    return matchesSearch && matchesFormat;
  });
});

// Lifecycle hooks
onMounted(() => {
  // Get video library
  getVideoLibrary();
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
      >视频库</h2>
      <p 
        :class="{
          'text-gray-400': currentTheme === 'dark',
          'text-gray-500': currentTheme === 'light'
        }"
      >浏览和管理已下载的视频文件</p>
    </div>
    
    <!-- Library Filters -->
    <div 
      class="rounded-lg p-4 shadow-lg mb-8"
      :class="{
        'bg-secondary': currentTheme === 'dark',
        'bg-white border border-gray-200': currentTheme === 'light'
      }"
    >
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div class="flex items-center space-x-4">
          <div class="relative">
            <input 
              type="text" 
              placeholder="搜索视频..." 
              class="rounded-lg py-2 pl-10 pr-4 focus:outline-none focus:ring-2 focus:ring-accent w-64"
              :class="{
                'bg-gray-700 text-white': currentTheme === 'dark',
                'bg-gray-100 text-gray-900 border border-gray-200': currentTheme === 'light'
              }"
              v-model="searchQuery"
            >
            <i 
              class="fa fa-search absolute left-3 top-3"
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            ></i>
          </div>
          
          <div class="relative">
            <select 
              class="rounded-lg py-2 pl-4 pr-10 appearance-none focus:outline-none focus:ring-2 focus:ring-accent"
              :class="{
                'bg-gray-700 text-white': currentTheme === 'dark',
                'bg-gray-100 text-gray-900 border border-gray-200': currentTheme === 'light'
              }"
              v-model="selectedFormat"
            >
              <option value="all">所有视频</option>
              <option value="mp4">MP4</option>
              <option value="mkv">MKV</option>
              <option value="avi">AVI</option>
              <option value="mov">MOV</option>
              <option value="wmv">WMV</option>
              <option value="flv">FLV</option>
              <option value="webm">WEBM</option>
              <option value="m4v">M4V</option>
              <option value="ts">TS</option>
            </select>
            <i 
              class="fa fa-chevron-down absolute right-3 top-3 pointer-events-none"
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            ></i>
          </div>
        </div>
        
        <div class="flex items-center space-x-2">
          <button 
            class="p-2 rounded-lg"
            :class="{
              'bg-gray-700 hover:bg-gray-600 text-white': currentTheme === 'dark',
              'bg-gray-100 hover:bg-gray-200 text-gray-900 border border-gray-200': currentTheme === 'light'
            }"
          >
            <i class="fa fa-th-large"></i>
          </button>
          <button 
            class="p-2 rounded-lg"
            :class="{
              'bg-gray-800 hover:bg-gray-700 text-white': currentTheme === 'dark',
              'bg-gray-100 hover:bg-gray-200 text-gray-900 border border-gray-200': currentTheme === 'light'
            }"
          >
            <i class="fa fa-list"></i>
          </button>
        </div>
      </div>
    </div>
    
    <!-- Video Grid -->
    <div v-if="isLoading" class="flex items-center justify-center py-10">
      <div class="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-accent"></div>
    </div>
    
    <div v-else-if="filteredVideos.length === 0" class="text-center py-10">
      <div 
        class="w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4"
        :class="{
          'bg-gray-700': currentTheme === 'dark',
          'bg-gray-100': currentTheme === 'light'
        }"
      >
        <i 
          class="fa fa-film text-2xl"
          :class="{
            'text-gray-500': currentTheme === 'dark',
            'text-gray-400': currentTheme === 'light'
          }"
        ></i>
      </div>
      <h3 
        class="text-lg font-semibold mb-2"
        :class="{
          'text-white': currentTheme === 'dark',
          'text-gray-900': currentTheme === 'light'
        }"
      >没有找到视频文件</h3>
      <p 
        :class="{
          'text-gray-400': currentTheme === 'dark',
          'text-gray-500': currentTheme === 'light'
        }"
      >请先下载一些视频文件</p>
    </div>
    
    <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6 mb-8">
      <div 
        v-for="video in filteredVideos" 
        :key="video.path" 
        class="video-card rounded-lg overflow-hidden shadow-lg"
        :class="{
          'bg-secondary': currentTheme === 'dark',
          'bg-white border border-gray-200': currentTheme === 'light'
        }"
      >
        <div class="relative cursor-pointer" @click="playVideo(video)">
          <div 
            class="w-full h-40 flex items-center justify-center relative overflow-hidden"
            :class="{
              'bg-gray-800': currentTheme === 'dark',
              'bg-gray-100': currentTheme === 'light'
            }"
          >
            <img 
              v-if="videoThumbnails[video.name] && !generatingThumbnails[video.name]"
              :src="videoThumbnails[video.name]" 
              :alt="video.name + ' Thumbnail'" 
              class="w-full h-full object-cover"
              @error="handleImageError"
            >
            <div 
              v-else-if="generatingThumbnails[video.name]" 
              class="flex flex-col items-center justify-center"
              :class="{
                'text-gray-400': currentTheme === 'dark',
                'text-gray-500': currentTheme === 'light'
              }"
            >
              <div class="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-accent mb-2"></div>
              <span class="text-xs">生成缩略图...</span>
            </div>
            <div 
              v-else 
              class="flex flex-col items-center justify-center"
              :class="{
                'text-gray-500': currentTheme === 'dark',
                'text-gray-400': currentTheme === 'light'
              }"
            >
              <i class="fa fa-film text-3xl mb-2"></i>
              <span class="text-xs">{{ video.extension.toUpperCase() }}</span>
            </div>
          </div>
          <div class="absolute inset-0 bg-black bg-opacity-40 flex items-center justify-center opacity-0 hover:opacity-100 transition-opacity">
            <div class="w-14 h-14 bg-accent bg-opacity-80 rounded-full flex items-center justify-center">
              <i class="fa fa-play text-white text-xl"></i>
            </div>
          </div>
          <div class="absolute bottom-2 right-2 bg-black bg-opacity-70 text-white text-xs px-2 py-1 rounded">
            {{ formatFileSize(video.size) }}
          </div>
        </div>
        <div class="p-4">
          <h3 
            class="font-medium mb-1 truncate"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >{{ video.name }}</h3>
          <div 
            class="flex items-center justify-between text-xs"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >
            <span>{{ formatFileSize(video.size) }}</span>
            <span>{{ video.extension.toUpperCase() }}</span>
          </div>
          <div class="mt-3 flex justify-between">
            <button class="text-accent hover:text-accentLight" title="播放" @click="playVideo(video)">
              <i class="fa fa-play-circle"></i>
            </button>
            <button class="text-info hover:text-blue-400" title="转码">
              <i class="fa fa-exchange"></i>
            </button>
            <button 
              class="text-success hover:text-green-400 disabled:opacity-50 disabled:cursor-not-allowed" 
              title="下载" 
              @click="downloadVideo(video)"
              :disabled="downloadingVideos[video.name]"
            >
              <i :class="downloadingVideos[video.name] ? 'fa fa-spinner fa-spin' : 'fa fa-download'"></i>
            </button>
          </div>
        </div>
      </div>
    </div>
    
    <!-- Video Player Modal -->
    <div v-if="showVideoPlayer && currentVideo" class="fixed inset-0 bg-black bg-opacity-90 z-50 flex items-center justify-center p-4">
      <div class="relative w-full max-w-4xl">
        <button 
          class="absolute top-4 right-4 bg-black bg-opacity-50 hover:bg-opacity-70 text-white p-2 rounded-full z-10"
          @click="closeVideoPlayer"
        >
          <i class="fa fa-times text-xl"></i>
        </button>
        <!-- 使用Wails安全文件系统API访问本地视频 -->
        <video 
          :src="`/downloads/${currentVideo.name}`" 
          class="w-full rounded-lg shadow-2xl"
          controls
          autoplay
        ></video>
        <div class="mt-4 text-center">
          <h3 
            class="text-xl font-semibold"
            :class="{
              'text-white': currentTheme === 'dark',
              'text-gray-900': currentTheme === 'light'
            }"
          >{{ currentVideo.name }}</h3>
          <p 
            class="mt-1"
            :class="{
              'text-gray-400': currentTheme === 'dark',
              'text-gray-500': currentTheme === 'light'
            }"
          >{{ formatFileSize(currentVideo.size) }} • {{ currentVideo.extension.toUpperCase() }}</p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
/* Library specific styles */
.fade-in {
  animation: fadeIn 0.5s ease-in-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.video-card {
  transition: all 0.3s ease;
}

.video-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3);
}
</style>