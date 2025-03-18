package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 全局缓存和锁
var (
	dataCache      *AirQualityData
	cacheMutex     sync.RWMutex
	lastUpdated    time.Time
	updateInterval = 5 * time.Minute
)

var API = os.Getenv("API")

// 数据结构
type AirQualityData struct {
	Status string `json:"status"`
	Data   struct {
		Aqi         int    `json:"aqi"`
		Dominentpol string `json:"dominentpol"`
		City        struct {
			Name string    `json:"name"`
			Geo  []float64 `json:"geo"`
		} `json:"city"`
		Iaqi map[string]struct {
			V float64 `json:"v"`
		} `json:"iaqi"`
		Time struct {
			S string `json:"s"`
		} `json:"time"`
		Attributions []struct {
			Name string `json:"name"`
			Url  string `json:"url"`
		} `json:"attributions"`
		Forecast struct {
			Daily struct {
				Pm25 []struct {
					Day string  `json:"day"`
					Avg float64 `json:"avg"`
				} `json:"pm25"`
			} `json:"daily"`
		} `json:"forecast"`
	} `json:"data"`
}

type AirQualityView struct {
	Status       string
	Aqi          int
	Dominentpol  string
	CityName     string
	CityGeo      string
	Iaqi         map[string]float64
	Time         string
	Attributions []Attribution
	ForecastPm25 []ForecastPm25
}

type Attribution struct {
	Name string
	Url  string
}

type ForecastPm25 struct {
	Day string
	Avg float64
}

func main() {
	// 设置为生产环境
	gin.SetMode(gin.ReleaseMode)

	// 检查 API 环境变量
	if API == "" {
		log.Fatal("API 环境变量未设置")
	}

	// 初始化Gin
	r := gin.Default()

	// 自定义模板函数
	r.SetFuncMap(template.FuncMap{
		"formatGeo": func(geo []float64) string {
			return fmt.Sprintf("%.4f, %.4f", geo[0], geo[1])
		},
		"toJSON": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return template.JS(b)
		},
	})

	// 加载模板
	r.LoadHTMLGlob("templates/*")

	// 启动后台数据更新
	go updateData()

	// 路由设置
	r.GET("/", handleIndex)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 启动服务器
	log.Println("Server starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handleIndex(c *gin.Context) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	if dataCache == nil {
		c.HTML(http.StatusServiceUnavailable, "error.html", gin.H{
			"message": "数据正在初始化，请稍后刷新",
		})
		return
	}

	viewData := transform(dataCache)
	c.HTML(http.StatusOK, "index.html", gin.H{
		"Data":      viewData,
		"Timestamp": viewData.Time, // 使用 API 返回的时间
	})
}

func updateData() {
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	// 立即执行第一次更新
	if err := fetchData(); err != nil {
		log.Printf("Initial data fetch failed: %v", err)
	}

	for range ticker.C {
		if err := fetchData(); err != nil {
			log.Printf("Data update failed: %v", err)
		}
	}
}

func fetchData() error {
	resp, err := http.Get(API)
	if err != nil {
		return fmt.Errorf("API请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API返回非200状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	var newData AirQualityData
	if err := json.Unmarshal(body, &newData); err != nil {
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	cacheMutex.Lock()
	dataCache = &newData
	lastUpdated = time.Now()
	cacheMutex.Unlock()

	log.Println("数据更新成功")
	return nil
}

func transform(data *AirQualityData) AirQualityView {
	view := AirQualityView{
		Status:       data.Status,
		Aqi:          data.Data.Aqi,
		Dominentpol:  data.Data.Dominentpol,
		CityName:     data.Data.City.Name,
		CityGeo:      fmt.Sprintf("%.4f, %.4f", data.Data.City.Geo[0], data.Data.City.Geo[1]),
		Iaqi:         make(map[string]float64),
		Time:         data.Data.Time.S,
		Attributions: []Attribution{},
		ForecastPm25: []ForecastPm25{},
	}

	// 解析所有实时指标，例如 pm25、o3、no2 等
	for key, v := range data.Data.Iaqi {
		view.Iaqi[key] = v.V
	}

	// 解析属性信息
	for _, attr := range data.Data.Attributions {
		view.Attributions = append(view.Attributions, Attribution{
			Name: attr.Name,
			Url:  attr.Url,
		})
	}

	// 解析 PM2.5 预报数据
	for _, item := range data.Data.Forecast.Daily.Pm25 {
		view.ForecastPm25 = append(view.ForecastPm25, ForecastPm25{
			Day: item.Day,
			Avg: item.Avg,
		})
	}

	return view
}