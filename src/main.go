package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// Blog 自定义博客结构体
type Blog struct {
	ID          int       `json:"id"`                   // 博客ID
	Title       string    `json:"title"`                // 标题
	AuthorID    int       `json:"author_id"`            // 作者ID
	Content     string    `json:"content"`              // 内容
	Tags        []string  `json:"tags,omitempty"`       // 标签（可选）
	CreatedTime time.Time `json:"created_at"`           // 创建时间（自动生成）
	UpdatedTime time.Time `json:"updated_at"`           // 更新时间（自动生成）
	IsPublished bool      `json:"is_published"`         // 是否发布（默认false）
	ViewCount   int       `json:"view_count,omitempty"` // 浏览次数（可选）
}

// ApiResponse 响应结构体
type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// 博客存储目录
const blogDir = "data/blogs"

func init() {
	// 创建存储目录
	if err := os.MkdirAll(blogDir, 0755); err != nil {
		log.Fatalf("Failed to create blog directory: %v", err)
	}
}

// Save 保存博客到文件
func (b *Blog) Save() error {
	// 设置时间戳
	if b.CreatedTime.IsZero() {
		b.CreatedTime = time.Now()
	}
	b.UpdatedTime = time.Now()

	// 生成文件名
	filename := filepath.Join(blogDir, fmt.Sprintf("%d.json", b.ID))

	// 序列化为JSON
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal blog: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write blog file: %w", err)
	}

	return nil
}

// 加载博客
func LoadBlog(id int) (*Blog, error) {
	filename := filepath.Join(blogDir, fmt.Sprintf("%d.json", id))
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read blog file: %w", err)
	}

	var blog Blog
	if err := json.Unmarshal(data, &blog); err != nil {
		return nil, fmt.Errorf("failed to unmarshal blog: %w", err)
	}

	return &blog, nil
}

// 发送JSON响应
func sendResponse(w http.ResponseWriter, success bool, message string, data interface{}, errMsg string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ApiResponse{
		Success: success,
		Message: message,
		Data:    data,
		Error:   errMsg,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// 获取博客ID从URL路径
var blogIDPath = regexp.MustCompile("^/api/blogs/([0-9]+)$")

func getBlogID(r *http.Request) (int, error) {
	matches := blogIDPath.FindStringSubmatch(r.URL.Path)
	if matches == nil {
		return 0, fmt.Errorf("invalid blog ID path")
	}

	id, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid blog ID format")
	}

	return id, nil
}

// 获取博客处理器
func getBlogHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getBlogID(r)
	if err != nil {
		sendResponse(w, false, "", nil, err.Error(), http.StatusBadRequest)
		return
	}

	blog, err := LoadBlog(id)
	if err != nil {
		sendResponse(w, false, "", nil, "Blog not found", http.StatusNotFound)
		return
	}

	// 增加浏览次数
	blog.ViewCount++
	if err := blog.Save(); err != nil {
		log.Printf("Failed to update view count: %v", err)
	}

	sendResponse(w, true, "Blog retrieved successfully", blog, "", http.StatusOK)
}

// 创建/更新博客处理器
func saveBlogHandler(w http.ResponseWriter, r *http.Request) {
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendResponse(w, false, "", nil, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// 解析JSON
	var blog Blog
	if err := json.Unmarshal(body, &blog); err != nil {
		sendResponse(w, false, "", nil, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// 验证必要字段
	if blog.Title == "" {
		sendResponse(w, false, "", nil, "Title is required", http.StatusBadRequest)
		return
	}

	if blog.Content == "" {
		sendResponse(w, false, "", nil, "Content is required", http.StatusBadRequest)
		return
	}

	// 对于PUT请求，检查ID是否匹配URL
	if r.Method == http.MethodPut {
		id, err := getBlogID(r)
		if err != nil {
			sendResponse(w, false, "", nil, err.Error(), http.StatusBadRequest)
			return
		}
		if blog.ID != id {
			sendResponse(w, false, "", nil, "Blog ID mismatch", http.StatusBadRequest)
			return
		}
	} else {
		// 对于POST请求，生成新ID
		blog.ID = generateNewBlogID()
	}

	// 保存博客
	if err := blog.Save(); err != nil {
		sendResponse(w, false, "", nil, "Failed to save blog", http.StatusInternalServerError)
		return
	}

	sendResponse(w, true, "Blog saved successfully", blog, "", http.StatusOK)
}

// 生成新博客ID（简单实现）
func generateNewBlogID() int {
	files, err := os.ReadDir(blogDir)
	if err != nil {
		log.Printf("Failed to read blog directory: %v", err)
		return int(time.Now().Unix())
	}
	return len(files) + 1
}

func main() {
	// 注册路由
	http.HandleFunc("/api/blogs/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getBlogHandler(w, r)
		case http.MethodPost, http.MethodPut:
			saveBlogHandler(w, r)
		default:
			sendResponse(w, false, "", nil, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 启动服务器
	log.Println("Starting blog API server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
