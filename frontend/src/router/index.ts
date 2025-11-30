import { createRouter, createWebHashHistory } from "vue-router";
import DashboardView from "../views/DashboardView.vue";
import TorrentView from "../views/TorrentView.vue";
import DownloadsView from "../views/DownloadsView.vue";
import LibraryView from "../views/LibraryView.vue";
import TranscodeView from "../views/TranscodeView.vue";

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: "/",
      name: "dashboard",
      component: DashboardView,
      meta: { title: "仪表盘", icon: "dashboard" }
    },
    {
      path: "/torrent",
      name: "torrent",
      component: TorrentView,
      meta: { title: "种子解析", icon: "torrent" }
    },
    {
      path: "/downloads",
      name: "downloads",
      component: DownloadsView,
      meta: { title: "下载管理", icon: "download" }
    },
    {
      path: "/library",
      name: "library",
      component: LibraryView,
      meta: { title: "视频库", icon: "library" }
    },
    {
      path: "/transcode",
      name: "transcode",
      component: TranscodeView,
      meta: { title: "视频转码", icon: "transcode" }
    },
    // 404 兜底路由
    {
      path: "/:pathMatch(.*)*",
      redirect: "/"
    }
  ],
  // 滚动行为
  scrollBehavior() {
    return { top: 0 };
  }
});

export default router;