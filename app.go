package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"golang.org/x/sys/windows"
)

// TranscodeTask represents a video transcoding task
// TranscodeTask 表示视频转码任务
type TranscodeTask struct {
	TaskID        string    `json:"taskId"`
	InputFile     string    `json:"inputFile"`
	OutputFile    string    `json:"outputFile"`
	Status        string    `json:"status"` // waiting, transcoding, completed, failed, cancelled
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	Progress      float64   `json:"progress"`
	Speed         string    `json:"speed,omitempty"`
	TimeRemaining string    `json:"timeRemaining,omitempty"`
	FFmpegCommand string    `json:"ffmpegCommand"`
	PID           int       `json:"pid,omitempty"`
	Error         string    `json:"error,omitempty"`
	VideoCodec    string    `json:"videoCodec"`
	AudioCodec    string    `json:"audioCodec"`
	Resolution    string    `json:"resolution"`
	Bitrate       string    `json:"bitrate"`
}

// GPUType 表示GPU的类型
type GPUType string

const (
	GPUTypeNVIDIA GPUType = "nvidia"
	GPUTypeAMD    GPUType = "amd"
	GPUTypeIntel  GPUType = "intel"
	GPUTypeOther  GPUType = "other"
)

// HasGPU 检测系统是否有可用的GPU，并返回GPU类型
func HasGPU() (bool, GPUType) {
	// 在Windows系统上，使用wmic命令检测GPU
	cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "Name")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("检测GPU失败: %v\n", err)
		return false, GPUTypeOther
	}

	// 将输出转换为字符串并检查是否包含GPU信息
	outputStr := string(output)
	// 过滤掉标题行和空行
	outputStr = strings.TrimSpace(outputStr)
	lines := strings.Split(outputStr, "\n")

	// 如果有多行输出（标题行+至少一个GPU），则认为系统有GPU
	if len(lines) > 1 {
		gpuNames := lines[1:]
		fmt.Printf("检测到GPU: %s\n", strings.Join(gpuNames, ", "))

		// 检查GPU类型 - 增强AMD检测逻辑以更好地支持RX6400
		outputLower := strings.ToLower(outputStr)
		if strings.Contains(outputLower, "nvidia") {
			return true, GPUTypeNVIDIA
		} else if strings.Contains(outputLower, "amd") || strings.Contains(outputLower, "radeon") || strings.Contains(outputLower, "ati") ||
			strings.Contains(outputLower, "amd radeon") || strings.Contains(outputLower, "rx") {
			return true, GPUTypeAMD
		} else if strings.Contains(outputLower, "intel") || strings.Contains(outputLower, "hd graphics") || strings.Contains(outputLower, "uhd graphics") || strings.Contains(outputLower, "iris") {
			return true, GPUTypeIntel
		}
		return true, GPUTypeOther
	}

	fmt.Println("未检测到可用GPU")
	return false, GPUTypeOther
}

// CheckFFmpegGPU 检查ffmpeg是否支持GPU加速
func CheckFFmpegGPU(ffmpegPath string) bool {
	// 执行ffmpeg -hwaccels命令查看支持的硬件加速类型
	cmd := exec.Command(ffmpegPath, "-hwaccels")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("检查ffmpeg GPU支持失败: %v\n", err)
		return false
	}

	outputStr := string(output)
	outputLower := strings.ToLower(outputStr)
	// 检查输出中是否包含常见的GPU加速关键字
	gpuKeywords := []string{"cuda", "nvenc", "dxva2", "d3d11va", "qsv", "vulkan", "amf", "vce", "opencl"}
	for _, keyword := range gpuKeywords {
		if strings.Contains(outputLower, keyword) {
			fmt.Printf("ffmpeg支持GPU加速: %s\n", keyword)
			return true
		}
	}

	// 额外检查AMD VCE编码器支持
	encodersCmd := exec.Command(ffmpegPath, "-encoders")
	encodersCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	encodersOutput, _ := encodersCmd.CombinedOutput()
	encodersLower := strings.ToLower(string(encodersOutput))
	if strings.Contains(encodersLower, "h264_amf") || strings.Contains(encodersLower, "hevc_amf") {
		fmt.Println("ffmpeg支持AMD AMF编码器")
		return true
	}

	fmt.Println("ffmpeg不支持GPU加速")
	return false
}

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
// NewApp 创建一个新的 App 应用程序
func NewApp() *App {
	return &App{}
}

// startup is called at application startup
// startup 在应用程序启动时调用
func (a *App) startup(ctx context.Context) {
	// Perform your setup here
	// 在这里执行初始化设置
	a.ctx = ctx

	// 扫描下载进度文件，处理异常状态的任务
	fmt.Println("应用程序启动，开始扫描下载进度文件...")

	// 读取下载进度文件
	progressFile := "download_progress.json"
	data, err := os.ReadFile(progressFile)
	if err != nil {
		// 如果文件不存在，创建一个空的进度文件
		if os.IsNotExist(err) {
			fmt.Println("下载进度文件不存在，创建空文件")
			if err := os.WriteFile(progressFile, []byte("[]"), 0644); err != nil {
				fmt.Printf("创建下载进度文件失败: %v\n", err)
			}
			return
		}
		fmt.Printf("读取下载进度文件失败: %v\n", err)
		return
	}

	// 解析JSON数据
	var progressList []map[string]interface{}
	if err := json.Unmarshal(data, &progressList); err != nil {
		fmt.Printf("解析下载进度数据失败: %v\n", err)
		return
	}

	// 检查是否有正在下载的任务，如果有，将其状态改为等待中
	var hasDownloadingTask bool
	for i, task := range progressList {
		if status, ok := task["status"].(string); ok && status == "downloading" {
			fmt.Printf("发现异常下载中的任务: %s，将状态改为等待中\n", task["taskId"])
			progressList[i]["status"] = "waiting"
			progressList[i]["endTime"] = time.Now().Format(time.RFC3339)
			// 移除PID，因为进程可能已经结束
			delete(progressList[i], "pid")
			hasDownloadingTask = true
		}
	}

	// 如果有修改，写入更新后的进度信息
	if hasDownloadingTask {
		progressData, err := json.MarshalIndent(progressList, "", "  ")
		if err != nil {
			fmt.Printf("生成进度信息失败: %v\n", err)
			return
		}
		if err := os.WriteFile(progressFile, progressData, 0644); err != nil {
			fmt.Printf("写入进度文件失败: %v\n", err)
			return
		}
		fmt.Println("已将异常下载中的任务状态改为等待中")
	}

	// 查找最早的等待中的任务
	var earliestTask map[string]interface{}
	var earliestTime time.Time
	for _, task := range progressList {
		if status, ok := task["status"].(string); ok && status == "waiting" {
			// 解析开始时间
			startTimeStr, ok := task["startTime"].(string)
			if !ok {
				continue
			}
			startTime, err := time.Parse(time.RFC3339, startTimeStr)
			if err != nil {
				fmt.Printf("解析任务开始时间失败: %v\n", err)
				continue
			}
			// 找到最早的任务
			if earliestTask == nil || startTime.Before(earliestTime) {
				earliestTask = task
				earliestTime = startTime
			}
		}
	}

	// 如果有等待中的任务，启动最早的那个
	if earliestTask != nil {
		taskId := earliestTask["taskId"].(string)
		magnetLink := earliestTask["magnetLink"].(string)
		outputDir := earliestTask["outputDir"].(string)
		fmt.Printf("启动最早的等待中的任务: %s，开始时间: %s\n", taskId, earliestTime.Format(time.RFC3339))
		if err := a.startDownload(taskId, magnetLink, outputDir, progressFile); err != nil {
			fmt.Printf("启动等待任务失败: %v\n", err)
		}
	} else {
		fmt.Println("没有等待中的任务")
	}

	// 扫描转码进度文件，处理异常状态的转码任务
	fmt.Println("开始扫描转码进度文件...")

	// 读取转码进度文件
	transcodeProgressFile := "transcode_progress.json"
	transcodeData, err := os.ReadFile(transcodeProgressFile)
	if err != nil {
		// 如果文件不存在，创建一个空的转码进度文件
		if os.IsNotExist(err) {
			fmt.Println("转码进度文件不存在，创建空文件")
			if err := os.WriteFile(transcodeProgressFile, []byte("[]"), 0644); err != nil {
				fmt.Printf("创建转码进度文件失败: %v\n", err)
			}
		} else {
			fmt.Printf("读取转码进度文件失败: %v\n", err)
		}
		return
	}

	// 解析转码JSON数据
	var transcodeTasks []TranscodeTask
	if err := json.Unmarshal(transcodeData, &transcodeTasks); err != nil {
		fmt.Printf("解析转码进度数据失败: %v\n", err)
		return
	}

	// 检查是否有正在转码的任务，如果有，将其状态改为等待中
	var hasTranscodingTask bool
	for i, task := range transcodeTasks {
		if task.Status == "transcoding" {
			fmt.Printf("发现异常转码中的任务: %s，将状态改为等待中\n", task.TaskID)
			transcodeTasks[i].Status = "waiting"
			transcodeTasks[i].EndTime = time.Now()
			// 移除PID，因为进程可能已经结束
			transcodeTasks[i].PID = 0
			hasTranscodingTask = true
		}
	}

	// 如果有修改，写入更新后的转码进度信息
	if hasTranscodingTask {
		transcodeProgressData, err := json.MarshalIndent(transcodeTasks, "", "  ")
		if err != nil {
			fmt.Printf("生成转码进度信息失败: %v\n", err)
			return
		}
		if err := os.WriteFile(transcodeProgressFile, transcodeProgressData, 0644); err != nil {
			fmt.Printf("写入转码进度文件失败: %v\n", err)
			return
		}
		fmt.Println("已将异常转码中的任务状态改为等待中")
	}

	// 查找最早的等待中的转码任务
	var earliestTranscodeTask *TranscodeTask
	var earliestTranscodeTime time.Time
	for i, task := range transcodeTasks {
		if task.Status == "waiting" {
			if earliestTranscodeTask == nil || task.StartTime.Before(earliestTranscodeTime) {
				earliestTranscodeTask = &transcodeTasks[i]
				earliestTranscodeTime = task.StartTime
			}
		}
	}

	// 如果有等待中的转码任务，启动最早的那个
	if earliestTranscodeTask != nil {
		fmt.Printf("启动最早的等待中的转码任务: %s，开始时间: %s\n",
			earliestTranscodeTask.TaskID, earliestTranscodeTime.Format(time.RFC3339))
		if err := a.startTranscode(earliestTranscodeTask.TaskID, transcodeProgressFile); err != nil {
			fmt.Printf("启动等待的转码任务失败: %v\n", err)
		}
	} else {
		fmt.Println("没有等待中的转码任务")
	}
}

// domReady is called after the front-end dom has been loaded
// domReady 在前端Dom加载完毕后调用
func (a *App) domReady(ctx context.Context) {
	// Add your action here
	// 在这里添加你的操作
}

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue,
// false will continue shutdown as normal.
// beforeClose在单击窗口关闭按钮或调用runtime.Quit即将退出应用程序时被调用.
// 返回 true 将导致应用程序继续，false 将继续正常关闭。
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	// 调用shutdown函数终止所有下载进程
	a.shutdown(ctx)
	return false
}

// shutdown is called at application termination
// 在应用程序终止时被调用
func (a *App) shutdown(ctx context.Context) {
	// Perform your teardown here
	// 在此处做一些资源释放的操作
	fmt.Println("应用程序正在关闭，开始清理下载任务...")

	// 读取下载进度文件
	progressFile := "download_progress.json"
	data, err := os.ReadFile(progressFile)
	if err != nil {
		fmt.Printf("读取下载进度文件失败: %v\n", err)
		return
	}

	// 解析JSON数据
	var progressList []map[string]interface{}
	if err := json.Unmarshal(data, &progressList); err != nil {
		fmt.Printf("解析下载进度数据失败: %v\n", err)
		return
	}

	// 查找并终止所有下载中的任务
	for _, task := range progressList {
		status, ok := task["status"].(string)
		if !ok {
			continue
		}

		// 只处理下载中的任务
		if status == "downloading" {
			// 获取PID
			pidValue, ok := task["pid"]
			if !ok {
				continue
			}

			// 处理不同类型的PID值（可能是float64或int）
			var pid int
			switch v := pidValue.(type) {
			case float64:
				pid = int(v)
			case int:
				pid = v
			default:
				continue
			}

			// 获取任务ID用于日志
			taskId := "unknown"
			if id, ok := task["taskId"].(string); ok {
				taskId = id
			}

			fmt.Printf("正在终止下载任务 %s (PID: %d)\n", taskId, pid)

			// 尝试终止进程
			// 在Windows上，我们使用taskkill命令
			killCmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
			output, err := killCmd.CombinedOutput()
			if err != nil {
				fmt.Printf("终止进程 %d 时出错: %v\n输出: %s\n", pid, err, string(output))
			} else {
				fmt.Printf("成功终止进程 %d\n", pid)
			}
		}
	}

	fmt.Println("下载任务清理完成")
}

// TorrentFileInfo represents the information of a torrent file
// TorrentFileInfo 表示种子文件的信息
type TorrentFileInfo struct {
	Name         string        `json:"name"`
	Size         int64         `json:"size"`
	Files        []TorrentFile `json:"files"`
	InfoHash     string        `json:"infoHash"`
	CreatedBy    string        `json:"createdBy"`
	CreationDate int64         `json:"creationDate"`
	Comment      string        `json:"comment"`
	Announce     []string      `json:"announce"`
	AnnounceList [][]string    `json:"announceList"`
}

// TorrentFile represents a file in the torrent
// TorrentFile 表示种子中的一个文件
type TorrentFile struct {
	Path   []string `json:"path"`
	Length int64    `json:"length"`
	Name   string   `json:"name"`
}

// FileInfo represents detailed information about a file in the torrent
// FileInfo 表示种子中文件的详细信息
type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// TorrentInfoResponse represents the response for torrent parsing
// TorrentInfoResponse 表示种子解析的响应
type TorrentInfoResponse struct {
	TotalSize int64      `json:"totalSize"`
	Files     []FileInfo `json:"files"`
	FileName  string     `json:"fileName"`
}

// ParseTorrentFile parses a torrent file and returns its information
// ParseTorrentFile 解析种子文件并返回其信息
func (a *App) ParseTorrentFile(fileData string) (string, error) {
	// 解析前端传递的JSON数据
	type FileRequest struct {
		Content  string `json:"content"`
		FileName string `json:"fileName"`
	}

	var req FileRequest
	if err := json.Unmarshal([]byte(fileData), &req); err != nil {
		return "", err
	}

	// 解码Base64字符串为字节数组
	data, err := base64.StdEncoding.DecodeString(req.Content)
	if err != nil {
		return "", err
	}

	// 加载并解析种子
	mi, err := metainfo.Load(bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	// 解析 Info 部分（文件信息核心）
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return "", err
	}

	// 提取文件列表和总大小
	fmt.Printf("IsDir: %v\n", info.IsDir())

	// 初始化文件信息切片和总大小
	fileInfos := make([]FileInfo, 0)
	totalSize := int64(0)

	if info.IsDir() {
		// 多文件种子
		for _, f := range info.Files {
			// 构建完整文件路径
			// 获取文件名（路径的最后一部分）
			fileName := f.Path[len(f.Path)-1]
			// 创建文件信息结构体
			fileInfo := FileInfo{
				Name: fileName, // 文件名
				Size: f.Length, // 文件大小

			}
			// 添加到文件信息切片
			fileInfos = append(fileInfos, fileInfo)
			// 累加总大小
			totalSize += f.Length
		}
	} else {
		// 单文件种子
		// 创建文件信息结构体
		fileInfo := FileInfo{
			Name: info.Name,   // 文件名
			Size: info.Length, // 文件大小

		}
		// 添加到文件信息切片
		fileInfos = append(fileInfos, fileInfo)
		// 设置总大小
		totalSize = info.Length
	}

	// 创建响应结构体
	torrentInfoResponse := TorrentInfoResponse{
		TotalSize: totalSize,    // 整个种子的总大小
		Files:     fileInfos,    // 包含每个文件详细信息的切片
		FileName:  req.FileName, // 传入的文件名称
	}

	// 打印调试信息
	fmt.Printf("Total Size: %v\n", totalSize)
	fmt.Printf("Files: %v\n", fileInfos)

	// 转换为JSON字符串
	jsonData, err := json.Marshal(torrentInfoResponse)
	if err != nil {
		return "", err
	}

	fmt.Printf("jsonData: %v\n", string(jsonData))
	return string(jsonData), nil
}

// GetTranscodeStatus gets the status of transcoding tasks
// GetTranscodeStatus 获取转码任务的状态
func (a *App) GetTranscodeStatus(taskID string) (string, error) {
	// 读取转码进度文件
	progressFile := "transcode_progress.json"
	data, err := os.ReadFile(progressFile)
	if err != nil {
		// 如果文件不存在或读取失败，返回空列表
		response := map[string]interface{}{
			"tasks": []TranscodeTask{},
		}
		jsonData, _ := json.Marshal(response)
		return string(jsonData), nil
	}

	// 解析JSON数据
	var transcodeTasks []TranscodeTask
	if err := json.Unmarshal(data, &transcodeTasks); err != nil {
		// 如果解析失败，返回空列表
		response := map[string]interface{}{
			"tasks": []TranscodeTask{},
		}
		jsonData, _ := json.Marshal(response)
		return string(jsonData), nil
	}

	// 如果taskID为空，返回所有任务状态
	if taskID == "" {
		response := map[string]interface{}{
			"tasks": transcodeTasks,
		}
		jsonData, err := json.Marshal(response)
		if err != nil {
			// 如果生成响应失败，返回空列表
			response := map[string]interface{}{
				"tasks": []TranscodeTask{},
			}
			jsonData, _ := json.Marshal(response)
			return string(jsonData), nil
		}
		return string(jsonData), nil
	}

	// 查找指定taskID的任务
	for _, task := range transcodeTasks {
		if task.TaskID == taskID {
			jsonData, err := json.Marshal(task)
			if err != nil {
				// 如果生成任务数据失败，返回空对象
				return "{}", nil
			}
			return string(jsonData), nil
		}
	}

	// 如果没有找到任务，返回空对象
	return "{}", nil
}

// CancelTranscode cancels a transcoding task
// CancelTranscode 取消转码任务
func (a *App) CancelTranscode(taskID string) (string, error) {
	fmt.Printf("取消转码任务: %s\n", taskID)

	// 读取转码进度文件
	progressFile := "transcode_progress.json"
	data, err := os.ReadFile(progressFile)
	if err != nil {
		return "", fmt.Errorf("读取转码进度文件失败: %w", err)
	}

	// 解析JSON数据
	var transcodeTasks []TranscodeTask
	if err := json.Unmarshal(data, &transcodeTasks); err != nil {
		return "", fmt.Errorf("解析转码进度数据失败: %w", err)
	}

	// 查找并取消指定taskID的任务
	var taskFound bool
	for i, task := range transcodeTasks {
		if task.TaskID == taskID {
			taskFound = true

			// 如果任务正在转码，杀死进程
			if task.Status == "transcoding" && task.PID != 0 {
				process, err := os.FindProcess(task.PID)
				if err != nil {
					fmt.Printf("查找进程 %d 时出错: %v\n", task.PID, err)
				} else {
					if err := process.Kill(); err != nil {
						fmt.Printf("终止进程 %d 时出错: %v\n", task.PID, err)
					} else {
						fmt.Printf("成功终止进程 %d\n", task.PID)
					}
				}
			}

			// 更新任务状态为已取消
			transcodeTasks[i].Status = "cancelled"
			transcodeTasks[i].EndTime = time.Now()
			break
		}
	}

	if !taskFound {
		return "", fmt.Errorf("未找到转码任务: %s", taskID)
	}

	// 写入更新后的进度信息
	updatedData, err := json.MarshalIndent(transcodeTasks, "", "  ")
	if err != nil {
		return "", fmt.Errorf("生成更新后的转码进度信息失败: %w", err)
	}
	if err := os.WriteFile(progressFile, updatedData, 0644); err != nil {
		return "", fmt.Errorf("写入转码进度文件失败: %w", err)
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Transcode cancelled successfully",
		"taskId":  taskID,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// DownloadTorrentFiles downloads selected files from a torrent
// DownloadTorrentFiles 从种子中下载选中的文件
func (a *App) DownloadTorrentFiles(fileData string, selectedFiles []string) (string, error) {
	// 解析前端传递的JSON数据
	fmt.Printf("开始下载种子文件，fileData: %s\n", fileData)
	fmt.Printf("选中的文件: %v\n", selectedFiles)

	type FileRequest struct {
		Content  string `json:"content"`
		FileName string `json:"fileName"`
	}

	var req FileRequest
	if err := json.Unmarshal([]byte(fileData), &req); err != nil {
		fmt.Printf("解析JSON失败: %v\n", err)
		return "", err
	}
	fmt.Printf("解析JSON成功，文件名: %s\n", req.FileName)

	// 解码Base64字符串为字节数组
	data, err := base64.StdEncoding.DecodeString(req.Content)
	if err != nil {
		fmt.Printf("Base64解码失败: %v\n", err)
		return "", err
	}
	fmt.Printf("Base64解码成功，数据长度: %d\n", len(data))

	// 创建临时文件保存种子内容
	tempFile, err := os.CreateTemp("", "*.torrent")
	if err != nil {
		fmt.Printf("创建临时文件失败: %v\n", err)
		return "", err
	}
	defer os.Remove(tempFile.Name())
	fmt.Printf("创建临时文件成功: %s\n", tempFile.Name())

	if _, err := tempFile.Write(data); err != nil {
		fmt.Printf("写入临时文件失败: %v\n", err)
		return "", err
	}
	if err := tempFile.Close(); err != nil {
		fmt.Printf("关闭临时文件失败: %v\n", err)
		return "", err
	}
	fmt.Printf("写入临时文件成功\n")

	// 获取可执行文件的绝对路径
	execPath, err := os.Getwd()
	if err != nil {
		fmt.Printf("获取工作目录失败: %v\n", err)
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}

	// 使用绝对路径的torrent命令
	torrentPath := filepath.Join(execPath, "tools", "torrent.exe")
	if _, err := os.Stat(torrentPath); os.IsNotExist(err) {
		fmt.Printf("torrent命令不存在: %v\n", err)
		return "", fmt.Errorf("torrent命令不存在: %w", err)
	}
	fmt.Printf("torrent命令存在: %s\n", torrentPath)

	// 调用torrent metainfo magnet命令生成磁力链接
	metainfoCmd := exec.Command(torrentPath, "metainfo", tempFile.Name(), "magnet")
	// 在Windows上隐藏命令窗口
	metainfoCmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	fmt.Printf("执行命令: %v\n", metainfoCmd.String())
	metainfoOutput, err := metainfoCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("执行torrent metainfo magnet命令失败: %v, 输出: %s\n", err, metainfoOutput)
		return "", fmt.Errorf("failed to get metainfo: %w, output: %s", err, metainfoOutput)
	}
	fmt.Printf("执行torrent metainfo magnet命令成功，输出: %s\n", metainfoOutput)

	// 解析磁力链接
	magnetLink := string(metainfoOutput)
	magnetLink = strings.TrimSpace(magnetLink)
	fmt.Printf("生成的磁力链接: %s\n", magnetLink)

	// 确保下载目录存在
	outputDir := "./downloads"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("创建下载目录失败: %v\n", err)
		return "", err
	}
	fmt.Printf("下载目录: %s\n", outputDir)

	// 创建进度文件
	progressFile := "download_progress.json"
	fmt.Printf("进度文件: %s\n", progressFile)

	// 初始化进度信息
	taskId := "task-" + fmt.Sprintf("%d", time.Now().Unix())
	initialProgress := map[string]interface{}{
		"taskId":        taskId,
		"magnetLink":    magnetLink,
		"status":        "waiting", // 默认状态为等待
		"totalSize":     0,
		"downloaded":    0,
		"selectedFiles": selectedFiles,
		"fileName":      req.FileName,
		"startTime":     time.Now().Format(time.RFC3339),
		"outputDir":     outputDir,
		"speed":         0,
		"percentage":    0,
	}

	// 读取现有进度文件，如果不存在则创建
	var progressList []map[string]interface{}
	existingData, err := os.ReadFile(progressFile)
	if err == nil {
		// 文件存在，解析现有数据
		if err := json.Unmarshal(existingData, &progressList); err != nil {
			fmt.Printf("解析现有进度文件失败: %v\n", err)
			progressList = []map[string]interface{}{}
		}
	} else {
		// 文件不存在，初始化空列表
		progressList = []map[string]interface{}{}
	}

	// 检查是否有正在下载的任务
	var hasDownloadingTask bool
	for _, task := range progressList {
		if status, ok := task["status"].(string); ok && status == "downloading" {
			hasDownloadingTask = true
			break
		}
	}

	// 添加新任务到进度列表
	progressList = append(progressList, initialProgress)

	// 写入初始进度信息
	progressData, err := json.MarshalIndent(progressList, "", "  ")
	if err != nil {
		fmt.Printf("生成进度信息失败: %v\n", err)
		return "", err
	}
	if err := os.WriteFile(progressFile, progressData, 0644); err != nil {
		fmt.Printf("写入进度文件失败: %v\n", err)
		return "", err
	}
	fmt.Printf("写入初始进度文件成功\n")

	// 如果没有正在下载的任务，立即开始下载当前任务
	if !hasDownloadingTask {
		// 调用内部下载函数开始下载
		if err := a.startDownload(taskId, magnetLink, outputDir, progressFile); err != nil {
			return "", err
		}
	} else {
		fmt.Printf("已有任务在下载中，当前任务 %s 进入等待状态\n", taskId)
	}

	// 构建响应
	response := map[string]interface{}{
		"status":        "success",
		"message":       "Download task added successfully",
		"taskId":        taskId,
		"magnetLink":    magnetLink,
		"selectedFiles": selectedFiles,
		"outputDir":     outputDir,
		"progressFile":  progressFile,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// AddTranscodeTaskWithParams adds a new transcoding task with custom FFmpeg parameters
// AddTranscodeTaskWithParams 添加带有自定义FFmpeg参数的新转码任务
func (a *App) AddTranscodeTaskWithParams(inputFile string, outputFile string, videoCodec string, audioCodec string, resolution string, bitrate string, ffmpegParams string) (string, error) {
	fmt.Printf("添加转码任务: %s -> %s\n", inputFile, outputFile)

	// 验证输入文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return "", fmt.Errorf("输入文件不存在: %s", inputFile)
	}

	// 获取可执行文件的绝对路径
	execPath, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}

	// 验证ffmpeg是否存在
	ffmpegPath := filepath.Join(execPath, "tools", "ffmpeg", "ffmpeg.exe")
	if _, err := os.Stat(ffmpegPath); os.IsNotExist(err) {
		return "", fmt.Errorf("ffmpeg不存在: %s", ffmpegPath)
	}

	// 创建转码任务
	taskID := "transcode-" + fmt.Sprintf("%d", time.Now().Unix())

	// 构建ffmpeg命令
	var ffmpegArgs []string
	var ffmpegCommand string

	// 如果提供了自定义FFmpeg参数，优先使用
	if ffmpegParams != "" {
		// 解析自定义FFmpeg参数
		customArgs := strings.Fields(ffmpegParams)
		// 构建完整的命令：ffmpeg -i inputFile [customParams] outputFile
		ffmpegArgs = append([]string{"-i", inputFile}, customArgs...)
		ffmpegArgs = append(ffmpegArgs, outputFile)
		ffmpegCommand = fmt.Sprintf("ffmpeg %s", strings.Join(ffmpegArgs, " "))
	} else {
		// 使用默认参数构建命令
		ffmpegArgs = []string{"-i", inputFile}

		// 添加视频编码器设置
		if videoCodec != "" {
			ffmpegArgs = append(ffmpegArgs, "-c:v", videoCodec)
		}

		// 添加音频编码器设置
		if audioCodec != "" {
			ffmpegArgs = append(ffmpegArgs, "-c:a", audioCodec)
		}

		// 添加分辨率设置
		if resolution != "" {
			ffmpegArgs = append(ffmpegArgs, "-s", resolution)
		}

		// 添加比特率设置
		if bitrate != "" {
			ffmpegArgs = append(ffmpegArgs, "-b:v", bitrate)
		}

		// 添加输出文件
		ffmpegArgs = append(ffmpegArgs, outputFile)

		ffmpegCommand = fmt.Sprintf("ffmpeg %s", strings.Join(ffmpegArgs, " "))
	}

	fmt.Printf("转码命令: %s\n", ffmpegCommand)

	// 创建转码任务
	transcodeTask := TranscodeTask{
		TaskID:        taskID,
		InputFile:     inputFile,
		OutputFile:    outputFile,
		Status:        "waiting",
		StartTime:     time.Now(),
		Progress:      0,
		FFmpegCommand: ffmpegCommand,
		VideoCodec:    videoCodec,
		AudioCodec:    audioCodec,
		Resolution:    resolution,
		Bitrate:       bitrate,
	}

	// 读取现有转码进度文件
	transcodeProgressFile := "transcode_progress.json"
	var transcodeTasks []TranscodeTask

	existingData, err := os.ReadFile(transcodeProgressFile)
	if err == nil {
		// 文件存在，解析现有数据
		if err := json.Unmarshal(existingData, &transcodeTasks); err != nil {
			fmt.Printf("解析现有转码进度文件失败: %v\n", err)
			transcodeTasks = []TranscodeTask{}
		}
	} else {
		// 文件不存在，初始化空列表
		transcodeTasks = []TranscodeTask{}
	}

	// 检查是否有正在转码的任务
	var hasRunningTask bool
	for _, task := range transcodeTasks {
		if task.Status == "transcoding" {
			hasRunningTask = true
			break
		}
	}

	// 添加新任务到列表
	transcodeTasks = append(transcodeTasks, transcodeTask)

	// 写入转码进度文件
	transcodeData, err := json.MarshalIndent(transcodeTasks, "", "  ")
	if err != nil {
		return "", fmt.Errorf("生成转码进度信息失败: %w", err)
	}
	if err := os.WriteFile(transcodeProgressFile, transcodeData, 0644); err != nil {
		return "", fmt.Errorf("写入转码进度文件失败: %w", err)
	}

	// 如果没有正在转码的任务，启动新任务
	if !hasRunningTask {
		if err := a.startNextTranscodeTask(transcodeProgressFile); err != nil {
			return "", fmt.Errorf("启动转码任务失败: %w", err)
		}
	} else {
		fmt.Printf("已有正在转码的任务，新任务将进入等待队列: %s\n", taskID)
	}

	// 构建响应
	response := map[string]interface{}{
		"status":     "success",
		"message":    "Transcode task added successfully",
		"taskId":     taskID,
		"inputFile":  inputFile,
		"outputFile": outputFile,
		"taskStatus": "waiting",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// AddTranscodeTask adds a new transcoding task
// AddTranscodeTask 添加新的转码任务
func (a *App) AddTranscodeTask(inputFile string, outputFile string, videoCodec string, audioCodec string, resolution string, bitrate string) (string, error) {
	// 调用带有默认FFmpeg参数的方法
	return a.AddTranscodeTaskWithParams(inputFile, outputFile, videoCodec, audioCodec, resolution, bitrate, "")
}

// startTranscode starts a transcoding task and monitors its progress
// startTranscode 开始转码任务并监控其进度
func (a *App) startTranscode(taskID string, progressFile string) error {
	fmt.Printf("开始执行转码任务: %s\n", taskID)

	// 读取转码进度文件
	data, err := os.ReadFile(progressFile)
	if err != nil {
		return fmt.Errorf("读取转码进度文件失败: %w", err)
	}

	var transcodeTasks []TranscodeTask
	if err := json.Unmarshal(data, &transcodeTasks); err != nil {
		return fmt.Errorf("解析转码进度数据失败: %w", err)
	}

	// 查找指定taskID的任务
	var taskIndex int
	var found bool
	for i, t := range transcodeTasks {
		if t.TaskID == taskID {
			taskIndex = i
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到转码任务: %s", taskID)
	}

	// 获取可执行文件的绝对路径
	execPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取工作目录失败: %w", err)
	}

	// 使用绝对路径的ffmpeg命令
	ffmpegPath := filepath.Join(execPath, "tools", "ffmpeg", "ffmpeg.exe")
	if _, err := os.Stat(ffmpegPath); os.IsNotExist(err) {
		return fmt.Errorf("ffmpeg不存在: %w", err)
	}

	// 直接使用任务的输入输出文件和参数重新构建命令，避免解析错误
	task := &transcodeTasks[taskIndex]
	var ffmpegArgs []string

	// 获取输出文件格式
	outputExt := filepath.Ext(task.OutputFile)
	if outputExt != "" {
		outputExt = outputExt[1:] // 移除点号
	}

	// 根据输出格式选择合适的编码器
	var videoCodec string
	var audioCodec string

	// 优先使用任务中指定的编码器
	if task.VideoCodec != "" {
		videoCodec = task.VideoCodec
	} else {
		// 根据输出格式选择默认视频编码器
		switch outputExt {
		case "webm":
			videoCodec = "libvpx-vp9"
		default:
			videoCodec = "libx264"
		}
	}

	if task.AudioCodec != "" {
		audioCodec = task.AudioCodec
	} else {
		// 根据输出格式选择默认音频编码器
		switch outputExt {
		case "webm":
			audioCodec = "libopus"
		default:
			audioCodec = "aac"
		}
	}

	// 检测系统是否有GPU以及ffmpeg是否支持GPU加速
	hasGPU, gpuType := HasGPU()
	ffmpegSupportsGPU := CheckFFmpegGPU(ffmpegPath)
	useGPU := hasGPU && ffmpegSupportsGPU

	fmt.Printf("GPU转码状态: 系统有GPU=%v, GPU类型=%v, ffmpeg支持GPU=%v, 是否使用GPU=%v\n", hasGPU, gpuType, ffmpegSupportsGPU, useGPU)

	// 启用GPU加速支持，不再强制使用CPU编码
	fmt.Println("启用GPU加速支持")

	// 确保在CPU模式下使用CPU编码器
	if !useGPU {
		// 重置为CPU编码器
		switch outputExt {
		case "webm":
			videoCodec = "libvpx-vp9"
		default:
			videoCodec = "libx264"
		}
		fmt.Printf("CPU模式下使用编码器: %s\n", videoCodec)
	}

	// 根据是否使用GPU设置不同的编码器和参数
	var hwaccelType string
	var gpuPreset string
	if useGPU {
		// GPU转码配置
		fmt.Println("使用GPU进行转码加速")

		// 检测GPU类型并设置合适的硬件加速参数
		// 基于GPU类型进行优先级检测
		switch gpuType {
		case GPUTypeNVIDIA:
			// NVIDIA GPU - 检查CUDA支持
			cudaCheckCmd := exec.Command(ffmpegPath, "-hwaccel", "cuda", "-i", task.InputFile, "-f", "null", "-")
			cudaCheckCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			_, cudaErr := cudaCheckCmd.CombinedOutput()

			if cudaErr == nil {
				// NVIDIA GPU
				fmt.Println("检测到NVIDIA GPU，使用CUDA加速")
				hwaccelType = "cuda"
				gpuPreset = "p4"

				// 设置GPU编码器，优先使用nvenc
				if outputExt == "mp4" || outputExt == "mkv" {
					// 对于H.264输出使用h264_nvenc
					if strings.ToLower(videoCodec) == "libx264" {
						videoCodec = "h264_nvenc"
					} else if strings.ToLower(videoCodec) == "libx265" {
						// 对于H.265输出使用hevc_nvenc
						videoCodec = "hevc_nvenc"
					}
				}
			} else {
				// 尝试使用DirectX作为备选
				d3dCheckCmd := exec.Command(ffmpegPath, "-hwaccel", "d3d11va", "-i", task.InputFile, "-f", "null", "-")
				d3dCheckCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				_, d3dErr := d3dCheckCmd.CombinedOutput()

				if d3dErr == nil {
					fmt.Println("NVIDIA GPU但CUDA不支持，使用DirectX加速")
					hwaccelType = "d3d11va"
				} else {
					fmt.Println("NVIDIA GPU但不支持特定加速，使用优化的CPU编码")
					useGPU = false
				}
			}
		case GPUTypeAMD:
			// AMD GPU - 增强AMF检测和优化，特别针对RX6400系列
			amfCheckCmd := exec.Command(ffmpegPath, "-encoders")
			amfCheckCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			amfOutput, _ := amfCheckCmd.CombinedOutput()
			amfOutputLower := strings.ToLower(string(amfOutput))
			fmt.Println("AMD GPU编码器检测输出:", amfOutputLower)

			// 检查是否支持AMD AMF编码器
			hasH264AMF := strings.Contains(amfOutputLower, "h264_amf")
			hasHEVCAMF := strings.Contains(amfOutputLower, "hevc_amf")
			fmt.Printf("AMD GPU AMF编码器支持: H264=%v, HEVC=%v\n", hasH264AMF, hasHEVCAMF)

			// 针对AMD RX6400优化的AMF配置
			if hasH264AMF || hasHEVCAMF {
				fmt.Println("检测到AMD GPU (可能是RX6400)，使用AMF加速")
				hwaccelType = "d3d11va" // AMD使用d3d11va作为硬件解码
				gpuPreset = "balanced"  // RX6400优化的预设

				// 强制设置AMF编码器，不管输入的是什么编码器
				if outputExt == "mp4" || outputExt == "mkv" {
					// 优先使用H.264 AMF
					if hasH264AMF {
						videoCodec = "h264_amf"
						fmt.Println("选择H.264 AMF编码器，适合AMD RX6400")
					} else if hasHEVCAMF {
						// 如果没有H.264 AMF再尝试HEVC AMF
						videoCodec = "hevc_amf"
						fmt.Println("选择HEVC AMF编码器，适合AMD RX6400")
					}
				}
			} else {
				// 尝试使用DirectX作为备选
				d3dCheckCmd := exec.Command(ffmpegPath, "-hwaccel", "d3d11va", "-i", task.InputFile, "-f", "null", "-")
				d3dCheckCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				d3dOutput, d3dErr := d3dCheckCmd.CombinedOutput()
				fmt.Println("DirectX加速检测输出:", string(d3dOutput))

				if d3dErr == nil {
					fmt.Println("AMD GPU但AMF不支持，使用DirectX加速")
					hwaccelType = "d3d11va"
				} else {
					fmt.Println("AMD GPU但不支持特定加速，使用优化的CPU编码")
					useGPU = false
				}
			}
		case GPUTypeIntel:
			// Intel GPU - 检查QSV支持
			qsvCheckCmd := exec.Command(ffmpegPath, "-hwaccel", "qsv", "-i", task.InputFile, "-f", "null", "-")
			qsvCheckCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			_, qsvErr := qsvCheckCmd.CombinedOutput()

			if qsvErr == nil {
				// Intel GPU
				fmt.Println("检测到Intel GPU，使用QSV加速")
				hwaccelType = "qsv"
				gpuPreset = "veryfast"

				if outputExt == "mp4" || outputExt == "mkv" {
					// 对于H.264输出使用h264_qsv
					if strings.ToLower(videoCodec) == "libx264" {
						videoCodec = "h264_qsv"
					} else if strings.ToLower(videoCodec) == "libx265" {
						// 对于H.265输出使用hevc_qsv
						videoCodec = "hevc_qsv"
					}
				}
			} else {
				// 尝试使用DirectX作为备选
				d3dCheckCmd := exec.Command(ffmpegPath, "-hwaccel", "d3d11va", "-i", task.InputFile, "-f", "null", "-")
				d3dCheckCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				_, d3dErr := d3dCheckCmd.CombinedOutput()

				if d3dErr == nil {
					fmt.Println("Intel GPU但QSV不支持，使用DirectX加速")
					hwaccelType = "d3d11va"
				} else {
					fmt.Println("Intel GPU但不支持特定加速，使用优化的CPU编码")
					useGPU = false
				}
			}
		default:
			// 其他GPU类型 - 尝试DirectX加速
			d3dCheckCmd := exec.Command(ffmpegPath, "-hwaccel", "d3d11va", "-i", task.InputFile, "-f", "null", "-")
			d3dCheckCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			_, d3dErr := d3dCheckCmd.CombinedOutput()

			if d3dErr == nil {
				fmt.Println("未知GPU类型，使用DirectX加速")
				hwaccelType = "d3d11va"
			} else {
				fmt.Println("未知GPU类型且不支持DirectX加速，使用优化的CPU编码")
				useGPU = false
			}
		}
	} else {
		// CPU转码配置（原配置）
		fmt.Println("使用CPU进行转码")
	}

	// 重新开始构建参数列表，确保正确的ffmpeg顺序
	ffmpegArgs = []string{}

	// 1. 添加硬件加速参数（必须放在-i之前）
	if useGPU && hwaccelType != "" {
		ffmpegArgs = append(ffmpegArgs, "-hwaccel", hwaccelType)
	}

	// 2. 添加输入文件
	ffmpegArgs = append(ffmpegArgs, "-i", task.InputFile)

	// 3. 添加视频编码器
	ffmpegArgs = append(ffmpegArgs, "-c:v", videoCodec)

	// 4. 添加GPU或CPU编码参数
	if useGPU {
		fmt.Printf("使用GPU编码，编码器: %s, GPU类型: %v\n", videoCodec, gpuType)

		// 根据GPU类型和编码器添加对应的参数
		switch {
		case strings.Contains(videoCodec, "nvenc"):
			// NVIDIA NVENC特有参数
			if gpuPreset != "" {
				ffmpegArgs = append(ffmpegArgs, "-preset", gpuPreset)
			} else {
				ffmpegArgs = append(ffmpegArgs, "-preset", "p4")
			}
			ffmpegArgs = append(ffmpegArgs, "-tune", "hq")
		case strings.Contains(videoCodec, "amf"):
			// AMD AMF特有参数，为RX6400优化
			if !strings.Contains(strings.Join(ffmpegArgs, " "), "-quality") {
				// 为RX6400选择优化的质量设置
				ffmpegArgs = append(ffmpegArgs, "-quality", "balanced")
			}
			// 添加RX6400的性能优化参数
			if !strings.Contains(strings.Join(ffmpegArgs, " "), "-rc") {
				ffmpegArgs = append(ffmpegArgs, "-rc", "cbr_hq")
				fmt.Println("为AMD RX6400添加优化参数: 使用高质量CBR编码")
			}
			// 为RX6400添加GOP设置
			if !strings.Contains(strings.Join(ffmpegArgs, " "), "-g") {
				ffmpegArgs = append(ffmpegArgs, "-g", "250")
				fmt.Println("为AMD RX6400添加优化参数: GOP大小设为250")
			}
		case strings.Contains(videoCodec, "qsv"):
			// Intel QSV特有参数
			if gpuPreset != "" {
				ffmpegArgs = append(ffmpegArgs, "-preset", gpuPreset)
			} else {
				ffmpegArgs = append(ffmpegArgs, "-preset", "veryfast")
			}
			ffmpegArgs = append(ffmpegArgs, "-look_ahead", "1")
		default:
			// 如果使用GPU但编码器不是GPU编码器，回退到CPU参数
			fmt.Println("警告: 使用GPU但编码器不是GPU编码器，使用CPU参数")
			ffmpegArgs = append(ffmpegArgs, "-preset", "medium", "-threads", "4")
		}
	} else {
		// CPU编码参数
		fmt.Println("使用CPU编码")
		ffmpegArgs = append(ffmpegArgs, "-preset", "medium", "-threads", "4")
	}

	// 5. 添加分辨率参数（如果指定）
	if task.Resolution != "" && task.Resolution != "original" {
		// 分辨率映射表，将常见分辨率名称转换为FFmpeg支持的像素尺寸
		resolutionMap := map[string]string{
			"1080p": "1920x1080",
			"720p":  "1280x720",
			"480p":  "854x480",
			"360p":  "640x360",
			"240p":  "426x240",
		}

		// 获取实际的像素尺寸
		actualResolution := task.Resolution
		if mapped, ok := resolutionMap[task.Resolution]; ok {
			actualResolution = mapped
		}

		ffmpegArgs = append(ffmpegArgs, "-s", actualResolution)
	}

	// 6. 添加比特率参数（如果指定）
	if task.Bitrate != "" {
		ffmpegArgs = append(ffmpegArgs, "-b:v", task.Bitrate)
	}

	// 7. 添加音频编码器
	ffmpegArgs = append(ffmpegArgs, "-c:a", audioCodec)

	// 8. 添加通用参数
	ffmpegArgs = append(ffmpegArgs, "-max_muxing_queue_size", "1024")
	if !useGPU {
		// 只有CPU模式需要限制滤镜线程数
		ffmpegArgs = append(ffmpegArgs, "-filter_threads", "2")
	}

	// 添加进度输出参数，让FFmpeg输出转码进度
	// -y: 覆盖输出文件
	// -progress pipe:1: 将进度信息输出到标准输出
	// -stats: 显示编码统计信息
	ffmpegArgs = append(ffmpegArgs, "-y", "-progress", "pipe:1", "-stats")

	// 添加输出文件
	ffmpegArgs = append(ffmpegArgs, task.OutputFile)

	transcodeCmd := exec.Command(ffmpegPath, ffmpegArgs...)
	// 在Windows上隐藏命令窗口
	transcodeCmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	fmt.Printf("执行转码命令: %v\n", transcodeCmd.String())

	// 获取命令的输出管道
	stdout, err := transcodeCmd.StdoutPipe()
	if err != nil {
		fmt.Printf("获取转码命令标准输出管道失败: %v\n", err)
		return err
	}

	stderr, err := transcodeCmd.StderrPipe()
	if err != nil {
		fmt.Printf("获取转码命令错误输出管道失败: %v\n", err)
		return err
	}

	// 启动命令
	if err := transcodeCmd.Start(); err != nil {
		fmt.Printf("启动转码命令失败: %v\n", err)
		return err
	}
	fmt.Printf("启动转码命令成功，进程ID: %d\n", transcodeCmd.Process.Pid)

	// 更新任务状态为转码中
	transcodeTasks[taskIndex].Status = "transcoding"
	transcodeTasks[taskIndex].PID = transcodeCmd.Process.Pid
	transcodeTasks[taskIndex].StartTime = time.Now()

	// 写入更新后的进度信息
	progressData, err := json.MarshalIndent(transcodeTasks, "", "  ")
	if err != nil {
		fmt.Printf("生成转码进度信息失败: %v\n", err)
		return err
	}
	if err := os.WriteFile(progressFile, progressData, 0644); err != nil {
		fmt.Printf("写入转码进度文件失败: %v\n", err)
		return err
	}

	// 在后台goroutine中监控转码进度，同时处理标准输出和标准错误
	go a.monitorTranscodeProgress(taskID, transcodeCmd, stdout, stderr, progressFile)

	return nil
}

// monitorTranscodeProgress monitors the progress of a transcoding task
// monitorTranscodeProgress 监控转码任务的进度
func (a *App) monitorTranscodeProgress(taskID string, cmd *exec.Cmd, stdout io.ReadCloser, stderr io.ReadCloser, progressFile string) {
	fmt.Printf("开始监控转码任务进度: %s\n", taskID)

	// 用于存储当前进度信息
	var currentProgress float64
	var totalDuration float64
	var hasTotalDuration bool
	var currentTime float64
	var currentFrame int64

	// 用于存储转码速度信息
	var currentSpeed string

	// 读取stdout输出以获取进度信息
	stdoutScanner := bufio.NewScanner(stdout)
	go func() {
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			fmt.Printf("FFmpeg输出: %s\n", line)

			// 解析FFmpeg progress信息（来自-progress参数，每行一个字段）
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := parts[0]
			value := parts[1]

			switch key {
			case "out_time":
				// 解析当前时间，例如：out_time=00:00:55.138417
				timeParts := strings.Split(value, ":")
				if len(timeParts) == 3 {
					hours, _ := strconv.ParseFloat(timeParts[0], 64)
					minutes, _ := strconv.ParseFloat(timeParts[1], 64)
					seconds, _ := strconv.ParseFloat(timeParts[2], 64)
					currentTime = hours*3600 + minutes*60 + seconds
				}
			// 忽略out_time_ms和out_time_us字段，只使用out_time字段计算进度
			// case "out_time_ms":
			// case "out_time_us":
			case "frame":
				// 解析当前帧数，例如：frame=48
				frame, err := strconv.ParseInt(value, 10, 64)
				if err == nil {
					currentFrame = frame
				}
			case "speed":
				// 解析转码速度信息，例如：speed=4.62x
				currentSpeed = value
				// 更新转码速度
				a.updateTranscodeSpeed(taskID, progressFile, currentSpeed)
			case "progress":
				// 解析进度状态，例如：progress=continue 或 progress=end
				if value == "end" {
					// 转码结束，进度设为1.0
					currentProgress = 1.0
					a.updateTranscodeProgress(taskID, progressFile, currentProgress, "")
					fmt.Printf("转码结束，进度设为100%%\n")
				}
			}

			// 计算进度
			var calculatedProgress float64
			var progressType string

			// 1. 优先使用时间计算进度（如果有总时长）
			if hasTotalDuration && totalDuration > 0 && currentTime > 0 {
				// 确保currentTime不超过总时长
				if currentTime > totalDuration {
					currentTime = totalDuration
				}
				calculatedProgress = currentTime / totalDuration
				progressType = "时间计算"
			} else if currentFrame > 0 {
				// 2. 使用帧数计算进度（每1000帧对应1%进度，避免一直为0）
				calculatedProgress = float64(currentFrame) / 100000.0
				// 限制帧数进度不超过99%
				if calculatedProgress > 0.99 {
					calculatedProgress = 0.99
				}
				progressType = "帧数计算"
			} else {
				// 3. 默认递增进度（0.1%递增，避免一直为0）
				calculatedProgress = currentProgress + 0.001
				// 限制默认进度不超过99%
				if calculatedProgress > 0.99 {
					calculatedProgress = 0.99
				}
				progressType = "默认递增"
			}

			// 确保进度在0-1之间
			if calculatedProgress < 0 {
				calculatedProgress = 0
			} else if calculatedProgress > 1 {
				calculatedProgress = 1
			}

			// 只有当进度有明显增加时才更新（避免频繁更新）
			if calculatedProgress > currentProgress+0.005 || calculatedProgress == 1.0 {
				currentProgress = calculatedProgress
				a.updateTranscodeProgress(taskID, progressFile, currentProgress, "")
				fmt.Printf("%s进度: 当前帧=%d, 当前时间=%.2f秒, 进度=%.2f%%\n", progressType, currentFrame, currentTime, currentProgress*100)
			}
		}
		if err := stdoutScanner.Err(); err != nil {
			// 忽略正常的管道关闭错误
			if !strings.Contains(err.Error(), "file already closed") {
				fmt.Printf("读取ffmpeg标准输出时出错: %v\n", err)
			}
		}
	}()

	// 尝试获取视频总时长（在stderr中）
	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stderrScanner.Scan() {
			line := stderrScanner.Text()
			fmt.Printf("FFmpeg错误输出: %s\n", line)

			// 检查是否有错误信息
			if strings.Contains(line, "Error") || strings.Contains(line, "error") {
				// 解析错误信息并更新任务状态，保持当前进度不变
				a.updateTranscodeProgress(taskID, progressFile, currentProgress, line)
				continue
			}

			// 尝试从stderr中获取总时长信息
			if !hasTotalDuration {
				durationRegex := regexp.MustCompile(`Duration: (\d+):(\d+):(\d+\.\d+)`)
				durationMatches := durationRegex.FindStringSubmatch(line)
				if len(durationMatches) == 4 {
					// 计算总时长（秒）
					hours, _ := strconv.ParseFloat(durationMatches[1], 64)
					minutes, _ := strconv.ParseFloat(durationMatches[2], 64)
					seconds, _ := strconv.ParseFloat(durationMatches[3], 64)
					totalDuration = hours*3600 + minutes*60 + seconds
					hasTotalDuration = true
					fmt.Printf("获取到总时长: %.2f秒\n", totalDuration)

					// 如果已经有当前时间信息，计算并更新进度
					if currentTime > 0 {
						currentProgress = currentTime / totalDuration
						if currentProgress > 1.0 {
							currentProgress = 1.0
						}
						a.updateTranscodeProgress(taskID, progressFile, currentProgress, "")
						fmt.Printf("使用时间计算进度: 当前时间=%.2f秒, 总时长=%.2f秒, 进度=%.2f%%\n", currentTime, totalDuration, currentProgress*100)
					}
				}
			}
		}
		if err := stderrScanner.Err(); err != nil {
			// 忽略正常的管道关闭错误
			if !strings.Contains(err.Error(), "file already closed") {
				fmt.Printf("读取ffmpeg错误输出时出错: %v\n", err)
			}
		}
	}()

	// 等待命令完成
	cmdErr := cmd.Wait()

	// 读取最终的进度信息
	data, readErr := os.ReadFile(progressFile)
	if readErr != nil {
		fmt.Printf("读取转码进度文件失败: %v\n", readErr)
		return
	}

	var transcodeTasks []TranscodeTask
	if err := json.Unmarshal(data, &transcodeTasks); err != nil {
		fmt.Printf("解析转码进度数据失败: %v\n", err)
		return
	}

	// 查找并更新任务状态
	for i, task := range transcodeTasks {
		if task.TaskID == taskID {
			if cmdErr != nil {
				// 转码失败
				transcodeTasks[i].Status = "failed"
				// 如果已经有详细的错误信息（从FFmpeg输出中提取），则保留，否则使用cmdErr
				if transcodeTasks[i].Error == "" {
					transcodeTasks[i].Error = cmdErr.Error()
				}
				fmt.Printf("转码任务失败: %s, 错误: %v\n", taskID, cmdErr)
			} else {
				// 转码成功
				transcodeTasks[i].Status = "completed"
				transcodeTasks[i].Progress = 1.0
				fmt.Printf("转码任务完成: %s\n", taskID)
			}
			transcodeTasks[i].EndTime = time.Now()
			break
		}
	}

	// 写入更新后的进度信息
	progressData, err := json.MarshalIndent(transcodeTasks, "", "  ")
	if err != nil {
		fmt.Printf("生成转码进度信息失败: %v\n", err)
		return
	}
	if err := os.WriteFile(progressFile, progressData, 0644); err != nil {
		fmt.Printf("写入转码进度文件失败: %v\n", err)
		return
	}

	fmt.Printf("转码任务监控结束: %s\n", taskID)

	// 检查是否有等待中的转码任务
	a.startNextTranscodeTask(progressFile)
}

// updateTranscodeProgress updates the progress of a transcoding task
// updateTranscodeProgress 更新转码任务的进度
func (a *App) updateTranscodeProgress(taskID string, progressFile string, progress float64, errorMsg string) error {
	// 读取进度文件
	data, err := os.ReadFile(progressFile)
	if err != nil {
		return fmt.Errorf("读取转码进度文件失败: %w", err)
	}

	var transcodeTasks []TranscodeTask
	if err := json.Unmarshal(data, &transcodeTasks); err != nil {
		return fmt.Errorf("解析转码进度数据失败: %w", err)
	}

	// 查找并更新任务进度
	for i, task := range transcodeTasks {
		if task.TaskID == taskID {
			// 只有当progress >= 0时才更新进度，否则保持当前进度不变
			if progress >= 0 {
				transcodeTasks[i].Progress = progress
			}
			// 更新错误信息
			if errorMsg != "" {
				transcodeTasks[i].Error = errorMsg
			}
			break
		}
	}

	// 写入更新后的进度信息
	progressData, err := json.MarshalIndent(transcodeTasks, "", "  ")
	if err != nil {
		return fmt.Errorf("生成转码进度信息失败: %w", err)
	}
	if err := os.WriteFile(progressFile, progressData, 0644); err != nil {
		return fmt.Errorf("写入转码进度文件失败: %w", err)
	}

	return nil
}

// updateTranscodeSpeed updates the speed of a transcoding task
// updateTranscodeSpeed 更新转码任务的速度
func (a *App) updateTranscodeSpeed(taskID string, progressFile string, speed string) error {
	// 读取进度文件
	data, err := os.ReadFile(progressFile)
	if err != nil {
		return fmt.Errorf("读取转码进度文件失败: %w", err)
	}

	var transcodeTasks []TranscodeTask
	if err := json.Unmarshal(data, &transcodeTasks); err != nil {
		return fmt.Errorf("解析转码进度数据失败: %w", err)
	}

	// 查找并更新任务速度
	for i, task := range transcodeTasks {
		if task.TaskID == taskID {
			// 更新转码速度
			transcodeTasks[i].Speed = speed
			break
		}
	}

	// 写入更新后的进度信息
	progressData, err := json.MarshalIndent(transcodeTasks, "", "  ")
	if err != nil {
		return fmt.Errorf("生成转码进度信息失败: %w", err)
	}
	if err := os.WriteFile(progressFile, progressData, 0644); err != nil {
		return fmt.Errorf("写入转码进度文件失败: %w", err)
	}

	return nil
}

// startNextTranscodeTask starts the next waiting transcoding task
// startNextTranscodeTask 启动下一个等待中的转码任务，实现任务队列
func (a *App) startNextTranscodeTask(progressFile string) error {
	// 读取进度文件
	data, err := os.ReadFile(progressFile)
	if err != nil {
		fmt.Printf("读取转码进度文件失败: %v\n", err)
		return err
	}

	var transcodeTasks []TranscodeTask
	if err := json.Unmarshal(data, &transcodeTasks); err != nil {
		fmt.Printf("解析转码进度数据失败: %v\n", err)
		return err
	}

	// 检查是否有正在转码的任务
	var hasRunningTask bool
	for _, task := range transcodeTasks {
		if task.Status == "transcoding" {
			hasRunningTask = true
			break
		}
	}

	// 如果有正在转码的任务，不启动新任务
	if hasRunningTask {
		fmt.Printf("已有正在转码的任务，等待完成后再启动下一个\n")
		return nil
	}

	// 查找第一个等待中的任务
	var nextTaskID string
	for _, task := range transcodeTasks {
		if task.Status == "waiting" {
			nextTaskID = task.TaskID
			break
		}
	}

	// 如果有等待中的任务，启动第一个
	if nextTaskID != "" {
		fmt.Printf("启动下一个等待中的转码任务: %s\n", nextTaskID)
		if err := a.startTranscode(nextTaskID, progressFile); err != nil {
			fmt.Printf("启动等待的转码任务失败: %v\n", err)
			return err
		}
	} else {
		fmt.Printf("没有等待中的转码任务\n")
	}

	return nil
}

// startDownload starts a download task and monitors its progress
// startDownload 开始下载任务并监控其进度
func (a *App) startDownload(taskId string, magnetLink string, outputDir string, progressFile string) error {
	// 获取可执行文件的绝对路径
	execPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取工作目录失败: %w", err)
	}

	// 使用绝对路径的torrent命令
	torrentPath := filepath.Join(execPath, "tools", "torrent.exe")
	if _, err := os.Stat(torrentPath); os.IsNotExist(err) {
		return fmt.Errorf("torrent命令不存在: %w", err)
	}

	// 调用torrent download命令下载种子文件
	downloadCmd := exec.Command(torrentPath, "download", magnetLink)
	// 在Windows上隐藏命令窗口
	downloadCmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	// 设置工作目录为downloads文件夹
	downloadCmd.Dir = outputDir
	fmt.Printf("执行命令: %v, 工作目录: %s\n", downloadCmd.String(), outputDir)

	// 获取命令的输出管道
	stdout, err := downloadCmd.StdoutPipe()
	if err != nil {
		fmt.Printf("获取命令输出管道失败: %v\n", err)
		return err
	}

	// 启动命令
	if err := downloadCmd.Start(); err != nil {
		fmt.Printf("启动下载命令失败: %v\n", err)
		return err
	}
	fmt.Printf("启动下载命令成功，进程ID: %d\n", downloadCmd.Process.Pid)

	// 更新进度信息为下载中
	existingData, err := os.ReadFile(progressFile)
	if err != nil {
		fmt.Printf("读取进度文件失败: %v\n", err)
		return err
	}

	var progressList []map[string]interface{}
	if err := json.Unmarshal(existingData, &progressList); err != nil {
		fmt.Printf("解析进度文件失败: %v\n", err)
		return err
	}

	for i, p := range progressList {
		if p["taskId"] == taskId {
			progressList[i]["status"] = "downloading"
			progressList[i]["pid"] = downloadCmd.Process.Pid
			break
		}
	}

	// 写入更新后的进度信息
	progressData, err := json.MarshalIndent(progressList, "", "  ")
	if err != nil {
		fmt.Printf("生成进度信息失败: %v\n", err)
		return err
	}
	if err := os.WriteFile(progressFile, progressData, 0644); err != nil {
		fmt.Printf("写入进度文件失败: %v\n", err)
		return err
	}
	fmt.Printf("更新进度状态为下载中成功\n")

	// 启动异步线程监控下载进度
	go func() {
		// 创建扫描器读取命令输出
		scanner := bufio.NewScanner(stdout)
		// 正则表达式匹配进度行，支持各种时间格式和单位格式
		progressRegex := regexp.MustCompile(`(?:(\d+)m)?(\d+(?:\.\d+)?)s: (\d+) torrents, (\d+) infos, (\d+(?:\.\d+)?\s*\w+)/(\d+(?:\.\d+)?\s*\w+) ready, upload (\d+(?:\.\d+)?\s*\w+), download (\d+(?:\.\d+)?\s*\w+)/s`)
		// 注意：matches[1]是分钟部分（可能为空），matches[2]是秒部分

		for scanner.Scan() {
			line := scanner.Text()
			fmt.Printf("下载输出: %s\n", line)

			// 匹配进度行
			matches := progressRegex.FindStringSubmatch(line)
			if len(matches) == 9 {
				// 解析进度信息
				// matches[1]: 分钟部分（可能为空）
				// matches[2]: 秒部分
				// matches[3]: 种子数
				// matches[4]: 信息数
				// matches[5]: 已下载大小
				// matches[6]: 总大小
				// matches[7]: 上传大小
				// matches[8]: 下载速度

				// 解析已下载大小
				downloaded, err := parseFileSize(matches[5])
				if err != nil {
					fmt.Printf("解析已下载大小失败: %v\n", err)
					continue
				}

				// 解析总大小
				totalSize, err := parseFileSize(matches[6])
				if err != nil {
					fmt.Printf("解析总大小失败: %v\n", err)
					continue
				}

				// 解析下载速度
				speed, err := parseFileSize(matches[8])
				if err != nil {
					fmt.Printf("解析下载速度失败: %v\n", err)
					continue
				}

				// 计算下载百分比
				percentage := 0.0
				if totalSize > 0 {
					percentage = (float64(downloaded) / float64(totalSize)) * 100
				}

				// 读取最新的进度文件
				existingData, err := os.ReadFile(progressFile)
				if err != nil {
					fmt.Printf("读取进度文件失败: %v\n", err)
					continue
				}

				var currentProgressList []map[string]interface{}
				if err := json.Unmarshal(existingData, &currentProgressList); err != nil {
					fmt.Printf("解析进度文件失败: %v\n", err)
					continue
				}

				// 更新当前任务的进度
				for i, p := range currentProgressList {
					if p["taskId"] == taskId {
						currentProgressList[i]["downloaded"] = downloaded
						currentProgressList[i]["totalSize"] = totalSize
						currentProgressList[i]["speed"] = speed
						currentProgressList[i]["percentage"] = percentage
						currentProgressList[i]["lastUpdate"] = time.Now().Format(time.RFC3339)
						break
					}
				}

				// 写入更新后的进度信息
				updatedData, err := json.MarshalIndent(currentProgressList, "", "  ")
				if err != nil {
					fmt.Printf("生成更新后的进度信息失败: %v\n", err)
					continue
				}
				if err := os.WriteFile(progressFile, updatedData, 0644); err != nil {
					fmt.Printf("写入更新后的进度信息失败: %v\n", err)
					continue
				}

				fmt.Printf("更新进度成功: 已下载 %s/%s, 速度 %s/s, 百分比 %.2f%%\n", matches[5], matches[6], matches[8], percentage)
			}
		}

		// 命令执行完成，更新状态为完成
		if err := scanner.Err(); err != nil {
			fmt.Printf("读取命令输出失败: %v\n", err)
		}

		// 等待命令执行完成
		if err := downloadCmd.Wait(); err != nil {
			fmt.Printf("下载命令执行失败: %v\n", err)

			// 检查任务是否已经被取消
			existingData, err := os.ReadFile(progressFile)
			if err != nil {
				fmt.Printf("读取进度文件失败: %v\n", err)
				return
			}

			var currentProgressList []map[string]interface{}
			if err := json.Unmarshal(existingData, &currentProgressList); err != nil {
				fmt.Printf("解析进度文件失败: %v\n", err)
				return
			}

			// 检查任务状态，如果已经是cancelled或paused，则不更新为completed
			for i, p := range currentProgressList {
				if p["taskId"] == taskId {
					currentStatus, ok := p["status"].(string)
					if ok && (currentStatus == "cancelled" || currentStatus == "paused") {
						fmt.Printf("任务已被取消或暂停，不更新为completed\n")
						// 启动下一个等待中的任务
						a.startNextWaitingTask()
						return
					}
					// 下载线程异常结束，将任务状态改为等待中
					currentProgressList[i]["status"] = "waiting"
					currentProgressList[i]["endTime"] = time.Now().Format(time.RFC3339)
					// 移除PID，因为进程已经结束
					delete(currentProgressList[i], "pid")
					break
				}
			}

			// 写入更新后的进度信息
			updatedData, err := json.MarshalIndent(currentProgressList, "", "  ")
			if err != nil {
				fmt.Printf("生成更新后的进度信息失败: %v\n", err)
				return
			}
			if err := os.WriteFile(progressFile, updatedData, 0644); err != nil {
				fmt.Printf("写入更新后的进度信息失败: %v\n", err)
				return
			}
			fmt.Printf("下载线程异常结束，任务 %s 状态改为等待中\n", taskId)
			// 启动下一个等待中的任务
			a.startNextWaitingTask()
			return
		}

		// 只有当命令正常完成时，才更新状态为completed
		// 读取最新的进度文件
		existingData, err := os.ReadFile(progressFile)
		if err != nil {
			fmt.Printf("读取进度文件失败: %v\n", err)
			return
		}

		var currentProgressList []map[string]interface{}
		if err := json.Unmarshal(existingData, &currentProgressList); err != nil {
			fmt.Printf("解析进度文件失败: %v\n", err)
			return
		}

		// 更新当前任务的状态为完成
		for i, p := range currentProgressList {
			if p["taskId"] == taskId {
				// 再次检查状态，确保没有被其他操作修改
				currentStatus, ok := p["status"].(string)
				if ok && (currentStatus == "cancelled" || currentStatus == "paused") {
					fmt.Printf("任务已被取消或暂停，不更新为completed\n")
					// 启动下一个等待中的任务
					a.startNextWaitingTask()
					return
				}
				currentProgressList[i]["status"] = "completed"
				currentProgressList[i]["endTime"] = time.Now().Format(time.RFC3339)
				break
			}
		}

		// 写入更新后的进度信息
		updatedData, err := json.MarshalIndent(currentProgressList, "", "  ")
		if err != nil {
			fmt.Printf("生成更新后的进度信息失败: %v\n", err)
			return
		}
		if err := os.WriteFile(progressFile, updatedData, 0644); err != nil {
			fmt.Printf("写入更新后的进度信息失败: %v\n", err)
			return
		}

		fmt.Printf("下载完成，更新状态为completed\n")
		// 启动下一个等待中的任务
		a.startNextWaitingTask()
	}()

	return nil
}

// startNextWaitingTask starts the next waiting download task
// startNextWaitingTask 启动下一个等待中的下载任务
func (a *App) startNextWaitingTask() {
	// 读取进度文件
	progressFile := "download_progress.json"
	existingData, err := os.ReadFile(progressFile)
	if err != nil {
		fmt.Printf("读取进度文件失败: %v\n", err)
		return
	}

	var progressList []map[string]interface{}
	if err := json.Unmarshal(existingData, &progressList); err != nil {
		fmt.Printf("解析进度文件失败: %v\n", err)
		return
	}

	// 检查是否有正在下载的任务
	for _, task := range progressList {
		if status, ok := task["status"].(string); ok && status == "downloading" {
			fmt.Printf("已有任务在下载中，不启动新任务\n")
			return
		}
	}

	// 查找第一个等待中的任务
	var nextTask map[string]interface{}
	for _, task := range progressList {
		if status, ok := task["status"].(string); ok && status == "waiting" {
			nextTask = task
			break
		}
	}

	if nextTask != nil {
		// 启动该任务
		taskId := nextTask["taskId"].(string)
		magnetLink := nextTask["magnetLink"].(string)
		outputDir := nextTask["outputDir"].(string)
		fmt.Printf("启动等待中的任务: %s\n", taskId)
		if err := a.startDownload(taskId, magnetLink, outputDir, progressFile); err != nil {
			fmt.Printf("启动等待任务失败: %v\n", err)
		}
	} else {
		fmt.Printf("没有等待中的任务\n")
	}
}

// StartWaitingTask starts a specific waiting download task
// StartWaitingTask 启动指定的等待中的下载任务
func (a *App) StartWaitingTask(taskId string) (string, error) {
	// 读取进度文件
	progressFile := "download_progress.json"
	existingData, err := os.ReadFile(progressFile)
	if err != nil {
		return "", fmt.Errorf("读取进度文件失败: %w", err)
	}

	var progressList []map[string]interface{}
	if err := json.Unmarshal(existingData, &progressList); err != nil {
		return "", fmt.Errorf("解析进度文件失败: %w", err)
	}

	// 检查是否有正在下载的任务
	for _, task := range progressList {
		if status, ok := task["status"].(string); ok && status == "downloading" {
			return "", fmt.Errorf("已有任务在下载中，无法启动新任务")
		}
	}

	// 查找指定的等待中的任务
	var targetTask map[string]interface{}
	for _, task := range progressList {
		if task["taskId"] == taskId && task["status"] == "waiting" {
			targetTask = task
			break
		}
	}

	if targetTask == nil {
		return "", fmt.Errorf("未找到指定的等待中的任务: %s", taskId)
	}

	// 启动该任务
	magnetLink := targetTask["magnetLink"].(string)
	outputDir := targetTask["outputDir"].(string)
	fmt.Printf("手动启动等待中的任务: %s\n", taskId)
	if err := a.startDownload(taskId, magnetLink, outputDir, progressFile); err != nil {
		return "", fmt.Errorf("启动等待任务失败: %w", err)
	}

	// 构建响应
	response := map[string]interface{}{
		"status":  "success",
		"message": "Waiting task started successfully",
		"taskId":  taskId,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// GetDiskSpace gets the disk space information of the current drive
// GetDiskSpace 获取当前驱动器的磁盘空间信息
func (a *App) GetDiskSpace() (string, error) {
	// 获取当前工作目录
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前工作目录失败: %w", err)
	}

	// 获取当前驱动器
	drive := cwd[:2] // Windows驱动器格式为 "C:" 等

	// 获取磁盘空间信息（Windows系统）
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(windows.StringToUTF16Ptr(drive), &freeBytesAvailable, &totalNumberOfBytes, &totalNumberOfFreeBytes); err != nil {
		return "", fmt.Errorf("获取磁盘空间信息失败: %w", err)
	}

	// 计算可用空间和总空间
	available := freeBytesAvailable
	total := totalNumberOfBytes
	used := total - available

	// 构建响应
	response := map[string]interface{}{
		"status":    "success",
		"drive":     drive,
		"total":     total,
		"available": available,
		"used":      used,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// GetVideoLibrary gets the list of video files in the downloads directory
// GetVideoLibrary 获取下载目录中的视频文件列表
func (a *App) GetVideoLibrary() (string, error) {
	// 下载目录
	downloadDir := "./downloads"

	// 读取目录中的所有文件
	files, err := os.ReadDir(downloadDir)
	if err != nil {
		return "", fmt.Errorf("读取下载目录失败: %w", err)
	}

	// 视频文件扩展名列表
	videoExtensions := map[string]bool{
		".mp4":  true,
		".mkv":  true,
		".avi":  true,
		".mov":  true,
		".wmv":  true,
		".flv":  true,
		".webm": true,
		".m4v":  true,
		".ts":   true,
	}

	// 过滤视频文件
	var videoFiles []map[string]interface{}
	for _, file := range files {
		if !file.IsDir() {
			// 获取文件扩展名
			ext := strings.ToLower(filepath.Ext(file.Name()))
			if videoExtensions[ext] {
				// 获取文件信息
				fileInfo, err := file.Info()
				if err != nil {
					continue
				}

				// 获取文件的完整路径
				fullPath, err := filepath.Abs(filepath.Join(downloadDir, file.Name()))
				if err != nil {
					continue
				}

				// 添加到视频文件列表
				videoFiles = append(videoFiles, map[string]interface{}{
					"name":      file.Name(),
					"size":      fileInfo.Size(),
					"path":      fullPath,
					"extension": ext[1:], // 移除点号
					"modTime":   fileInfo.ModTime().Format(time.RFC3339),
				})
			}
		}
	}

	// 构建响应
	response := map[string]interface{}{
		"status":     "success",
		"videoFiles": videoFiles,
		"total":      len(videoFiles),
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// GenerateMagnetLink generates magnet link from torrent file using external tool
// GenerateMagnetLink 使用外部工具从种子文件生成磁力链接
func (a *App) GenerateMagnetLink(torrentFilePath string) (string, error) {
	fmt.Printf("Generating magnet link for: %s\n", torrentFilePath)

	// 获取可执行文件的绝对路径
	execPath, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}

	// 使用绝对路径的torrent命令
	torrentPath := filepath.Join(execPath, "tools", "torrent.exe")
	if _, err := os.Stat(torrentPath); os.IsNotExist(err) {
		return "", fmt.Errorf("torrent命令不存在: %w", err)
	}

	// 调用torrent metainfo magnet命令生成磁力链接
	metainfoCmd := exec.Command(torrentPath, "metainfo", torrentFilePath, "magnet")
	// 在Windows上隐藏命令窗口
	metainfoCmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	fmt.Printf("执行命令: %v\n", metainfoCmd.String())
	metainfoOutput, err := metainfoCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("执行torrent metainfo magnet命令失败: %v, 输出: %s\n", err, metainfoOutput)
		return "", fmt.Errorf("failed to get metainfo: %w, output: %s", err, metainfoOutput)
	}
	fmt.Printf("执行torrent metainfo magnet命令成功，输出: %s\n", metainfoOutput)

	// 解析磁力链接
	magnetLink := string(metainfoOutput)
	magnetLink = strings.TrimSpace(magnetLink)
	fmt.Printf("生成的磁力链接: %s\n", magnetLink)

	response := map[string]interface{}{
		"status":     "success",
		"magnetLink": magnetLink,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// parseFileSize 解析文件大小字符串为字节数，支持各种单位格式
func parseFileSize(sizeStr string) (int64, error) {
	// 移除空格
	sizeStr = strings.ReplaceAll(sizeStr, " ", "")
	// 正则表达式匹配数字和单位，支持广泛的格式
	// 支持：123, 123B, 123KB, 123kB, 123MB, 123.45MB, 1.2GB, 1G, 2.5K等
	sizeRegex := regexp.MustCompile(`^(\d+(?:\.\d+)?)([KkMmGgTtPp]?[Bb]?)$`)
	matches := sizeRegex.FindStringSubmatch(sizeStr)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	// 解析数字部分
	size, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	// 解析单位部分，统一转换为大写
	unit := "B"
	if len(matches) > 2 && matches[2] != "" {
		unit = strings.ToUpper(matches[2])
		// 处理简写情况，如K, M, G等，添加B后缀
		if len(unit) == 1 {
			unit += "B"
		}
	}

	// 转换为字节数
	var multiplier float64 = 1
	switch unit {
	case "B":
		multiplier = 1
	case "KB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	case "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "PB":
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	}

	// 计算并返回字节数
	return int64(size * multiplier), nil
}

// DownloadWithTool downloads torrent using external tool
// DownloadWithTool 使用外部工具下载种子
func (a *App) DownloadWithTool(magnetLink string, outputDir string) (string, error) {
	// TODO: Implement download using external tool
	// 实现使用外部工具下载种子
	fmt.Printf("Downloading from magnet link: %s\n", magnetLink)
	fmt.Printf("Output directory: %s\n", outputDir)

	// Mock response for now
	// 暂时返回模拟数据
	response := map[string]interface{}{
		"status":  "success",
		"message": "Download started successfully",
		"taskId":  "task-789012",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// GetDownloadStatus gets the status of a download task
// GetDownloadStatus 获取下载任务的状态
func (a *App) GetDownloadStatus(taskId string) (string, error) {
	// 读取下载进度文件
	progressFile := "download_progress.json"
	data, err := os.ReadFile(progressFile)
	if err != nil {
		// 如果文件不存在，返回空列表
		if os.IsNotExist(err) {
			response := map[string]interface{}{
				"tasks": []map[string]interface{}{},
			}
			jsonData, _ := json.Marshal(response)
			return string(jsonData), nil
		}
		return "", err
	}

	// 解析JSON数据
	var progressList []map[string]interface{}
	if err := json.Unmarshal(data, &progressList); err != nil {
		return "", err
	}

	// 如果taskId为空，返回所有任务状态
	if taskId == "" {
		response := map[string]interface{}{
			"tasks": progressList,
		}
		jsonData, err := json.Marshal(response)
		if err != nil {
			return "", err
		}
		return string(jsonData), nil
	}

	// 查找指定taskId的任务
	for _, task := range progressList {
		if task["taskId"] == taskId {
			jsonData, err := json.Marshal(task)
			if err != nil {
				return "", err
			}
			return string(jsonData), nil
		}
	}

	// 如果没有找到任务，返回空对象
	return "{}", nil
}

// ServeVideoFile serves a video file from the downloads directory
// ServeVideoFile 从下载目录提供视频文件访问
func (a *App) ServeVideoFile(fileName string) (string, error) {
	// 下载目录
	downloadDir := "./downloads"

	// 构建完整的文件路径
	filePath := filepath.Join(downloadDir, fileName)

	// 验证文件存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("文件不存在: %s", fileName)
	}

	// 返回文件路径，Wails运行时会处理安全的文件访问
	response := map[string]interface{}{
		"status":   "success",
		"filePath": filePath,
		"fileName": fileName,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// UploadFile handles file upload
// UploadFile 处理文件上传
func (a *App) UploadFile(fileData string) (string, error) {
	// 解析前端传递的JSON数据
	type UploadRequest struct {
		Content  string `json:"content"`
		FileName string `json:"fileName"`
	}

	var req UploadRequest
	if err := json.Unmarshal([]byte(fileData), &req); err != nil {
		return "", err
	}

	// 创建转码目录
	transcodeDir := "./transcode"
	if err := os.MkdirAll(transcodeDir, 0755); err != nil {
		return "", fmt.Errorf("创建转码目录失败: %w", err)
	}

	// 创建与文件名同名的子目录
	baseName := strings.TrimSuffix(req.FileName, filepath.Ext(req.FileName))
	videoSubDir := filepath.Join(transcodeDir, baseName)
	if err := os.MkdirAll(videoSubDir, 0755); err != nil {
		return "", fmt.Errorf("创建视频子目录失败: %w", err)
	}

	// 保存上传的文件到子目录中
	inputFilePath := filepath.Join(videoSubDir, req.FileName)

	// 立即返回响应，不等待文件保存完成
	response := map[string]interface{}{
		"status":     "success",
		"message":    "File uploaded successfully",
		"fileName":   req.FileName,
		"filePath":   inputFilePath,
		"subDirName": baseName,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	// 使用goroutine后台处理文件保存，不阻塞响应返回
	go func() {
		fmt.Printf("开始后台保存文件: %s\n", req.FileName)

		// 解码Base64字符串为字节数组
		data, err := base64.StdEncoding.DecodeString(req.Content)
		if err != nil {
			fmt.Printf("解码Base64失败: %v\n", err)
			return
		}

		// 使用缓冲写入优化大文件写入
		file, err := os.Create(inputFilePath)
		if err != nil {
			fmt.Printf("创建文件失败: %v\n", err)
			return
		}
		defer file.Close()

		// 使用4KB缓冲区写入文件
		writer := bufio.NewWriterSize(file, 4096)
		defer writer.Flush()

		if _, err := writer.Write(data); err != nil {
			fmt.Printf("写入文件失败: %v\n", err)
			return
		}

		fmt.Printf("文件保存成功: %s\n", inputFilePath)
	}()

	// 立即返回响应，提升前端体验
	return string(jsonData), nil
}

// StartTranscode starts transcoding for an uploaded file
// StartTranscode 开始转码已上传的文件
func (a *App) StartTranscode(transcodeData string) (string, error) {
	// 解析前端传递的JSON数据
	type TranscodeRequest struct {
		FileName     string `json:"fileName"`
		OutputFormat string `json:"outputFormat"`
		Resolution   string `json:"resolution"`
		Quality      int    `json:"quality"`
		VideoCodec   string `json:"videoCodec"`
		AudioCodec   string `json:"audioCodec"`
		FFmpegParams string `json:"ffmpegParams"`
	}

	var req TranscodeRequest
	if err := json.Unmarshal([]byte(transcodeData), &req); err != nil {
		return "", err
	}

	// 构建输入文件路径
	baseName := strings.TrimSuffix(req.FileName, filepath.Ext(req.FileName))
	videoSubDir := filepath.Join("./transcode", baseName)
	inputFilePath := filepath.Join(videoSubDir, req.FileName)

	// 验证输入文件是否存在
	if _, err := os.Stat(inputFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("输入文件不存在: %s", inputFilePath)
	}

	// 生成输出文件名
	outputFileName := fmt.Sprintf("%s_converted.%s", baseName, req.OutputFormat)
	outputFilePath := filepath.Join(videoSubDir, outputFileName)

	// 构建比特率
	bitrate := fmt.Sprintf("%dk", req.Quality*1000)

	// 调用转码方法，并传递FFmpeg参数
	return a.AddTranscodeTaskWithParams(inputFilePath, outputFilePath, req.VideoCodec, req.AudioCodec, req.Resolution, bitrate, req.FFmpegParams)
}

// CancelDownload cancels a download task
// CancelDownload 取消下载任务
func (a *App) CancelDownload(taskId string) (string, error) {
	fmt.Printf("Cancelling download task: %s\n", taskId)

	// 读取下载进度文件
	progressFile := "download_progress.json"
	data, err := os.ReadFile(progressFile)
	if err != nil {
		return "", fmt.Errorf("读取下载进度文件失败: %w", err)
	}

	// 解析JSON数据
	var progressList []map[string]interface{}
	if err := json.Unmarshal(data, &progressList); err != nil {
		return "", fmt.Errorf("解析下载进度数据失败: %w", err)
	}

	// 查找并取消指定taskId的任务
	var taskFound bool
	for i, task := range progressList {
		if task["taskId"] == taskId {
			taskFound = true

			// 如果任务正在下载，杀死进程
			if task["status"] == "downloading" && task["pid"] != nil {
				// 获取PID
				var pid int
				switch v := task["pid"].(type) {
				case float64:
					pid = int(v)
				case int:
					pid = v
				default:
					continue
				}

				// 尝试终止进程
				process, err := os.FindProcess(pid)
				if err != nil {
					fmt.Printf("查找进程 %d 时出错: %v\n", pid, err)
				} else {
					if err := process.Kill(); err != nil {
						fmt.Printf("终止进程 %d 时出错: %v\n", pid, err)
					} else {
						fmt.Printf("成功终止进程 %d\n", pid)
					}
				}
			}

			// 更新任务状态为已取消
			progressList[i]["status"] = "cancelled"
			progressList[i]["endTime"] = time.Now().Format(time.RFC3339)
			break
		}
	}

	if !taskFound {
		return "", fmt.Errorf("task not found: %s", taskId)
	}

	// 写入更新后的进度信息
	updatedData, err := json.MarshalIndent(progressList, "", "  ")
	if err != nil {
		return "", fmt.Errorf("生成更新后的进度信息失败: %w", err)
	}
	if err := os.WriteFile(progressFile, updatedData, 0644); err != nil {
		return "", fmt.Errorf("写入进度文件失败: %w", err)
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Download cancelled successfully",
		"taskId":  taskId,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}
