package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	// 导入wails相关包
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

// toolsFiles变量已移除，不再需要embed tools目录

// 自定义中间件，用于处理文件下载请求
func customMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 处理下载目录的文件请求
		if filepath.HasPrefix(r.URL.Path, "/downloads/") {
			filePath := filepath.Join(".", r.URL.Path)

			// 安全检查：确保文件在下载目录内
			absPath, err := filepath.Abs(filePath)
			if err != nil {
				http.Error(w, "Invalid file path", http.StatusBadRequest)
				return
			}

			absDownloadDir, err := filepath.Abs("./downloads")
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !filepath.HasPrefix(absPath, absDownloadDir) {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}

			// 提供文件
			http.ServeFile(w, r, absPath)
			return
		} else if filepath.HasPrefix(r.URL.Path, "/transcode/") {
			// 处理转码目录的文件请求
			filePath := filepath.Join(".", r.URL.Path)

			// 安全检查：确保文件在转码目录内
			absPath, err := filepath.Abs(filePath)
			if err != nil {
				http.Error(w, "Invalid file path", http.StatusBadRequest)
				return
			}

			absTranscodeDir, err := filepath.Abs("./transcode")
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !filepath.HasPrefix(absPath, absTranscodeDir) {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}

			// 提供文件
			http.ServeFile(w, r, absPath)
			return
		}

		// 其他请求继续使用默认处理
		next.ServeHTTP(w, r)
	})
}

// copyToolsDir函数已移除，不再需要复制tools目录内容

func main() {
	// 创建一个App结构体实例
	app := NewApp()
	var err error

	// 运行GPU检测测试
	fmt.Println("===== GPU识别测试 =====")
	// 由于我们正在测试CPU转码功能，暂时跳过GPU检测
	fmt.Println("当前模式: CPU编码模式")

	// 检查FFmpeg路径
	ffmpegPath := ".\tools\ffmpeg\bin\ffmpeg.exe"
	// 检查ffmpeg是否存在
	if _, err := os.Stat(ffmpegPath); os.IsNotExist(err) {
		// 如果指定路径不存在，尝试从系统PATH中查找
		fmt.Println("指定的FFmpeg路径不存在，尝试从系统PATH中查找...")
		ffmpegPath = "ffmpeg" // 尝试使用系统PATH中的ffmpeg
	}
	fmt.Printf("使用FFmpeg路径: %s\n", ffmpegPath)
	fmt.Println("===== GPU识别测试完成 =====")

	// 确保downloads目录存在
	if _, err := os.Stat("./downloads"); os.IsNotExist(err) {
		err := os.Mkdir("./downloads", 0755)
		if err != nil {
			log.Printf("创建downloads目录失败: %v\n", err)
		}
	}

	// 确保transcode目录存在
	if _, err := os.Stat("./transcode"); os.IsNotExist(err) {
		err := os.Mkdir("./transcode", 0755)
		if err != nil {
			log.Printf("创建transcode目录失败: %v\n", err)
		}
	}

	// 移除了复制tools目录的操作

	// Create application with options
	// 使用选项创建应用
	err = wails.Run(&options.App{
		Title:             "SeedParser",
		Width:             1400,
		Height:            900,
		MinWidth:          1000,
		MinHeight:         600,
		MaxWidth:          0, // 移除最大宽度限制，允许窗口最大化到全屏
		MaxHeight:         0, // 移除最大高度限制，允许窗口最大化到全屏
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: 255, G: 255, B: 255, A: 0},
		Menu:              nil,
		Logger:            nil,
		LogLevel:          logger.DEBUG,
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		OnBeforeClose:     app.beforeClose,
		OnShutdown:        app.shutdown,
		WindowStartState:  options.Normal,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Handler:    nil,
			Middleware: customMiddleware,
		},
		Bind: []interface{}{
			app,
		},
		// Windows platform specific options
		// Windows平台特定选项
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			WebviewUserDataPath:               "",
			WebviewBrowserPath:                "",
			Theme:                             windows.SystemDefault,
		},
		// Mac platform specific options
		// Mac平台特定选项
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
				HideToolbarSeparator:       false,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "Wails Template Vue",
				Message: "A Wails template based on Vue and Vue-Router",
				Icon:    icon,
			},
		},
		Linux: &linux.Options{
			Icon: icon,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
