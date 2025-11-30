<script setup lang="ts">
import { ref, computed, onMounted, provide } from 'vue';
import { useRoute, useRouter } from 'vue-router';

const route = useRoute();
const router = useRouter();

// Get current route name for active navigation
const currentRoute = computed(() => route.name);

// Navigation items
const navItems = [
  { name: 'dashboard', icon: 'fa-tachometer', label: '仪表盘' },
  { name: 'torrent', icon: 'fa-magnet', label: '种子解析' },
  { name: 'downloads', icon: 'fa-download', label: '下载管理' },
  { name: 'library', icon: 'fa-film', label: '视频库' },
  { name: 'transcode', icon: 'fa-exchange', label: '视频转码' },
];

// Theme management
const currentTheme = ref(localStorage.getItem('theme') || 'dark');

// Update theme function
const updateTheme = (theme: string) => {
  currentTheme.value = theme;
  localStorage.setItem('theme', theme);
  document.documentElement.setAttribute('data-theme', theme);
  document.documentElement.className = theme;
};

// Notification management
interface Notification {
  id: number;
  message: string;
  type: 'success' | 'error' | 'warning' | 'info';
  duration?: number;
}

const notifications = ref<Notification[]>([]);
let notificationId = 0;

// Add notification function
const addNotification = (message: string, type: Notification['type'] = 'info', duration = 3000) => {
  const id = ++notificationId;
  const notification = { id, message, type, duration };
  
  notifications.value.push(notification);
  
  // Auto remove notification after duration
  if (duration > 0) {
    setTimeout(() => {
      removeNotification(id);
    }, duration);
  }
  
  return id;
};

// Remove notification function
const removeNotification = (id: number) => {
  const index = notifications.value.findIndex(n => n.id === id);
  if (index > -1) {
    notifications.value.splice(index, 1);
  }
};

// Lifecycle hooks
onMounted(() => {
  // Initialize theme
  updateTheme(currentTheme.value);
});

// Expose theme variables and notifications to all components
provide('currentTheme', currentTheme);
provide('updateTheme', updateTheme);
provide('addNotification', addNotification);
provide('removeNotification', removeNotification);
</script>

<template>
  <div class="flex h-screen overflow-hidden" :class="{ 'bg-primary text-white': currentTheme === 'dark', 'bg-gray-50 text-gray-900': currentTheme === 'light' }">
    <!-- Sidebar -->
    <aside class="w-64 glass-effect flex flex-col" :class="{ 'bg-secondary border-r border-gray-700': currentTheme === 'dark', 'bg-white border-r border-gray-200 shadow-md': currentTheme === 'light' }">
      <!-- Logo -->
      <div class="p-4 flex items-center justify-center" :class="{ 'border-b border-gray-700': currentTheme === 'dark', 'border-b border-gray-200': currentTheme === 'light' }">
        <div class="flex items-center">
          <img src="https://p3-flow-imagex-sign.byteimg.com/tos-cn-i-a9rns2rl98/rc/pc/super_tool/df1a8d02dfce40c2bf13b0a173505711~tplv-a9rns2rl98-image.image?rcl=2025112814210559205FA290D717FC7CFD&rk3s=8e244e95&rrcfp=f06b921b&x-expires=1766902947&x-signature=e43keYVWXbftIEu61%2FwuKT56PZg%3D" alt="VideoTorrent Logo" class="w-10 h-10 mr-3">
          <h1 class="text-xl font-bold" :class="{ 'text-white': currentTheme === 'dark', 'text-gray-900': currentTheme === 'light' }">VideoTorrent</h1>
        </div>
      </div>
      
      <!-- Navigation -->
      <nav class="flex-1 overflow-y-auto scrollbar-hide py-4">
        <ul>
          <li class="mb-2" v-for="item in navItems" :key="item.name">
            <router-link 
              :to="{ name: item.name }" 
              class="sidebar-item flex items-center px-4 py-3 transition-all duration-200"
              :class="{
                'text-gray-300 hover:text-white': currentTheme === 'dark',
                'text-gray-600 hover:text-gray-900': currentTheme === 'light',
                'active': currentRoute === item.name
              }"
            >
              <i :class="['fa', item.icon, 'w-6 text-center mr-3 text-accent']"></i>
              <span>{{ item.label }}</span>
            </router-link>
          </li>
        </ul>
      </nav>
      
      <!-- Status -->
      <div class="p-4" :class="{ 'border-t border-gray-700': currentTheme === 'dark', 'border-t border-gray-200': currentTheme === 'light' }">
        <div class="flex items-center justify-between">
          <span class="text-sm" :class="{ 'text-gray-400': currentTheme === 'dark', 'text-gray-500': currentTheme === 'light' }">状态</span>
          <span class="text-sm text-accent flex items-center">
            <span class="w-2 h-2 bg-accent rounded-full mr-2"></span>
            运行中
          </span>
        </div>
        <div class="mt-2 text-xs" :class="{ 'text-gray-500': currentTheme === 'dark', 'text-gray-400': currentTheme === 'light' }">
          <div class="flex justify-between">
            <span>版本</span>
            <span>1.0.0</span>
          </div>
        </div>
      </div>
    </aside>
    
    <!-- Main Content -->
    <main class="flex-1 overflow-y-auto relative" :class="{ 'bg-primary': currentTheme === 'dark', 'bg-gray-50': currentTheme === 'light' }">
      <!-- Content Sections -->
      <div class="container mx-auto px-4 py-6">
        <router-view />
      </div>
      
      <!-- Notifications -->
      <div class="fixed top-4 right-4 z-50 space-y-2">
        <div 
          v-for="notification in notifications" 
          :key="notification.id"
          class="notification-toast flex items-center px-4 py-3 rounded-lg shadow-lg border backdrop-blur-sm min-w-80 transform transition-all duration-300 ease-in-out"
          :class="{
            'bg-green-900/90 border-green-500 text-green-100': notification.type === 'success' && currentTheme === 'dark',
            'bg-green-50 border-green-200 text-green-800': notification.type === 'success' && currentTheme === 'light',
            'bg-red-900/90 border-red-500 text-red-100': notification.type === 'error' && currentTheme === 'dark',
            'bg-red-50 border-red-200 text-red-800': notification.type === 'error' && currentTheme === 'light',
            'bg-yellow-900/90 border-yellow-500 text-yellow-100': notification.type === 'warning' && currentTheme === 'dark',
            'bg-yellow-50 border-yellow-200 text-yellow-800': notification.type === 'warning' && currentTheme === 'light',
            'bg-blue-900/90 border-blue-500 text-blue-100': notification.type === 'info' && currentTheme === 'dark',
            'bg-blue-50 border-blue-200 text-blue-800': notification.type === 'info' && currentTheme === 'light'
          }"
        >
          <i 
            class="mr-3 text-lg"
            :class="{
              'fa fa-check-circle': notification.type === 'success',
              'fa fa-times-circle': notification.type === 'error',
              'fa fa-exclamation-triangle': notification.type === 'warning',
              'fa fa-info-circle': notification.type === 'info'
            }"
          ></i>
          <span class="flex-1">{{ notification.message }}</span>
          <button 
            @click="removeNotification(notification.id)"
            class="ml-3 text-lg opacity-70 hover:opacity-100 transition-opacity"
          >
            <i class="fa fa-times"></i>
          </button>
        </div>
      </div>
    </main>
  </div>
</template>

<style lang="scss">
// Import Tailwind CSS utilities
@tailwind base;
@tailwind components;
@tailwind utilities;

// Custom styles
body {
  font-family: 'Inter', system-ui, sans-serif;
  overflow-x: hidden;
  transition: all 0.3s ease;
}

// Dark mode styles
body.dark {
  background-color: #0f172a;
  color: #f8fafc;
}

// Light mode styles
body.light {
  background-color: #f9fafb;
  color: #111827;
}

.sidebar-item {
  transition: all 0.2s ease;
}

// Dark mode sidebar item styles
.dark .sidebar-item:hover {
  background-color: rgba(16, 185, 129, 0.1);
  border-left: 3px solid #10b981;
}

.dark .sidebar-item.active {
  background-color: rgba(16, 185, 129, 0.2);
  border-left: 3px solid #10b981;
}

// Light mode sidebar item styles
.light .sidebar-item:hover {
  background-color: rgba(16, 185, 129, 0.1);
  border-left: 3px solid #10b981;
}

.light .sidebar-item.active {
  background-color: rgba(16, 185, 129, 0.2);
  border-left: 3px solid #10b981;
}

// Dark mode glass effect
.dark .glass-effect {
  background: rgba(30, 41, 59, 0.7);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  border: 1px solid rgba(255, 255, 255, 0.05);
}

// Light mode glass effect
.light .glass-effect {
  background: rgba(255, 255, 255, 0.7);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  border: 1px solid rgba(0, 0, 0, 0.05);
}

.scrollbar-hide::-webkit-scrollbar {
  display: none;
}

.scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
}

.hover-scale {
  transition: transform 0.2s ease-in-out;
}

.hover-scale:hover {
  transform: scale(1.02);
}

.progress-bar {
  transition: width 0.3s ease;
}
</style>
