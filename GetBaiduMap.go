package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// GetBaiduMap 定义
type GetBaiduMap struct {
	threadCount              int
	baiduMapServer           *BaiduMapServerInfo
	errorList                *DownloadErrorInfo
	listCapacity             int
	currentDownloadTimes     int
	channel                  chan int
	config                   *ConfigStruct
	downloadFlag             bool
	broadcastMessageCallback BroadcastMessageCallback
	jobStatus                JobStatus
}

// BaiduMapServerInfo 定义
type BaiduMapServerInfo struct {
	MinServerID     int
	MaxServerID     int
	CurrentServerID int
}

// MapProperties 定义
type MapProperties struct {
	zoomLevel int
	x, y      int64
}

// JobStatus 定义
type JobStatus struct {
	counter, total, errorCounter uint64
}

// DownloadParaStruct 定义
type DownloadParaStruct struct {
	minZoomLevel, maxZoomLevel int
	provinces                  string
}

// RectAreaStruct 定义
type RectAreaStruct struct {
	top    float64
	bottom float64
	left   float64
	right  float64
}

// BroadcastMessageCallback 定义
type BroadcastMessageCallback func(message string)

// NewGetBaiduMap 定义
func NewGetBaiduMap(config *ConfigStruct, broadcastMessageCallback BroadcastMessageCallback) *GetBaiduMap {
	instance := new(GetBaiduMap)

	instance.downloadFlag = false

	instance.config = config
	instance.listCapacity = config.ProcessListCapacity

	instance.baiduMapServer = &BaiduMapServerInfo{
		MinServerID: 0, MaxServerID: 3, CurrentServerID: 0,
	}
	instance.errorList = &DownloadErrorInfo{
		listCaption: config.ProcessErrorListCapacity,
	}
	instance.threadCount = config.AllowedThreadCount
	instance.channel = make(chan int, instance.threadCount)
	instance.broadcastMessageCallback = broadcastMessageCallback
	instance.Init()
	return instance
}

// Init 定义
func (instance *GetBaiduMap) Init() {
	instance.baiduMapServer.CurrentServerID = instance.baiduMapServer.MinServerID
	atomic.StoreUint64(&instance.jobStatus.counter, 0)
	atomic.StoreUint64(&instance.jobStatus.total, 0)
	atomic.StoreUint64(&instance.jobStatus.errorCounter, 0)
}

// getImageFromURL 定义
func (instance *GetBaiduMap) getImageFromURL(url *string) (content []byte, err error) {
	resp, err1 := http.Get(*url)
	if err1 != nil {
		err = err1
		return
	}
	defer resp.Body.Close()
	data, err2 := ioutil.ReadAll(resp.Body)
	if err2 != nil {
		err = err2
		return
	}
	resp.Body.Close()

	statusCode := resp.StatusCode
	if statusCode != 200 {
		message := fmt.Sprintf("StatusCode is error! URL: %s", *url)
		err = errors.New(message)
		return
	}

	if data != nil && len(data) > 4 {
		if data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
			content = data
		}
	}
	return
}

// WriteImageToFile 定义
func (instance *GetBaiduMap) WriteImageToFile(raw []byte, pathName string, fileName string) (err error) {
	if raw != nil {
		err = os.MkdirAll(pathName, 0777)
		if err != nil {
			return
		}
		err = ioutil.WriteFile(fileName, raw, 0644)
	}
	return
}

var urlTemplate = "http://online%d.map.bdimg.com/onlinelabel/?qt=tile&x=%d&y=%d&z=%d&styles=pl&scaler=1&udt=%s"

// downloadMap 定义
func (instance *GetBaiduMap) downloadAMapTile(jobPath *string, mapProperties *MapProperties) (err error) {
	if instance.baiduMapServer.CurrentServerID > instance.baiduMapServer.MaxServerID {
		instance.baiduMapServer.CurrentServerID = 0
	}
	url := fmt.Sprintf(urlTemplate, instance.baiduMapServer.CurrentServerID, mapProperties.x, mapProperties.y, mapProperties.zoomLevel, time.Now().Format("20060102"))
	instance.baiduMapServer.CurrentServerID++
	url = strings.Replace(url, "-", "M", 0)

	pathName := fmt.Sprintf("%s/%d/%d/", *jobPath, mapProperties.zoomLevel, mapProperties.x)
	fileName := fmt.Sprintf("%s/%d.png", pathName, mapProperties.y)

	var raw []byte
	raw, err = instance.getImageFromURL(&url)
	if err != nil {
		return
	}

	err = instance.WriteImageToFile(raw, pathName, fileName)
	return
}

// downloadMapBySlices 定义
func (instance *GetBaiduMap) downloadMapBySlices(jobPath *string, mapProperties []*MapProperties, c chan int, j *JobStatus) {
	errMapProperties := make([]*MapProperties, 0, instance.errorList.listCaption)
	for _, value := range mapProperties {
		if value.zoomLevel == 0 {
			return
		}
		for i := 0; i < 3; i++ {
			err := instance.downloadAMapTile(jobPath, value)
			if err == nil {
				atomic.AddUint64(&j.counter, 1)
				break
			}
			if i >= 2 {
				atomic.AddUint64(&j.errorCounter, 1)
				errMapProperties = append(errMapProperties, value)
				if len(errMapProperties) >= instance.errorList.listCaption {
					instance.errorList.Append(errMapProperties)
					errMapProperties = make([]*MapProperties, 0, instance.errorList.listCaption)
				}
				break
			}
			time.Sleep(10)
		}
	}
	instance.errorList.Append(errMapProperties)
	c <- 1
}

// createJobPath 定义
func (instance *GetBaiduMap) createJobPath() (jobPath string, err error) {
	relativePath, err := os.Getwd()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	tmpPath := fmt.Sprintf("%s/map/", relativePath)

	_, tmpErr := os.Stat(tmpPath)
	if tmpErr == nil {
		for i := 1; ; i++ {
			tmpPath = fmt.Sprintf("%s/map%d/", relativePath, i)
			_, tmpErr := os.Stat(tmpPath)
			if tmpErr != nil {
				break
			}
		}
	}

	err = os.MkdirAll(tmpPath, 0777)
	if err == nil {
		jobPath = tmpPath
	}
	return
}

// getDownloadingAreas 定义
func (instance *GetBaiduMap) getDownloadingAreas(provincesStr string) []RectAreaStruct {
	var tempRectAreas []RectAreaStruct
	provinces := strings.Split(provincesStr, ",")

	for _, value := range instance.config.ProvinceInformation {
		for _, province := range provinces {
			if province == value.province {
				var rect RectAreaStruct
				longitude := value.area.longitude
				latitude := value.area.latitude
				if longitude[0] < longitude[1] {
					rect.left = longitude[0]
					rect.right = longitude[1]
				} else {
					rect.left = longitude[1]
					rect.right = longitude[0]
				}
				if latitude[0] < latitude[1] {
					rect.top = latitude[0]
					rect.bottom = latitude[1]
				} else {
					rect.top = latitude[1]
					rect.bottom = latitude[0]
				}
				if tempRectAreas == nil {
					tempRectAreas = make([]RectAreaStruct, 1, 100)
					tempRectAreas[0] = rect
				} else {
					tempRectAreas = append(tempRectAreas, rect)
				}
			}
		}
	}
	validRectAreas := instance.UnionRectAreas(tempRectAreas)
	return validRectAreas
}

// ChenkPointInRectAreas 定义
func (instance *GetBaiduMap) ChenkPointInRectAreas(pointX float64, pointY float64, rectAreas []RectAreaStruct) (ret bool) {
	ret = false
	for _, rect := range rectAreas {
		if pointX >= rect.left && pointX <= rect.right && pointY >= rect.top && pointY <= rect.bottom {
			ret = true
			return
		}
	}
	return
}

// UnionRectAreas 定义
func (instance *GetBaiduMap) UnionRectAreas(rectAreas []RectAreaStruct) (validRectAreas []RectAreaStruct) {
	if rectAreas == nil {
		return
	}
	x := make([]float64, 0, len(rectAreas)*2)
	y := make([]float64, 0, len(rectAreas)*2)
	for _, value := range rectAreas {
		x = append(x, value.left)
		x = append(x, value.right)
		y = append(y, value.top)
		y = append(y, value.bottom)
	}

	sort.Float64s(x)
	sort.Float64s(y)

	for indexX := range x {
		if indexX+1 == len(x) {
			break
		}
		for indexY := range y {
			if indexY+1 == len(y) {
				break
			}
			pointX := (x[indexX] + x[indexX+1]) / 2
			pointY := (y[indexY] + y[indexY+1]) / 2
			if instance.ChenkPointInRectAreas(pointX, pointY, rectAreas) == true {
				rect := RectAreaStruct{y[indexY], y[indexY+1], x[indexX], x[indexX+1]}
				if validRectAreas == nil {
					validRectAreas = make([]RectAreaStruct, 1, 100)
					validRectAreas[0] = rect
				} else {
					validRectAreas = append(validRectAreas, rect)
				}
			}
		}
	}
	return
}

// FetchMaps 定义
func (instance *GetBaiduMap) fetchMaps(jobPath string, minZoom int, maxZoom int, rectAreas []RectAreaStruct) {
	instance.Init()
	counter := uint64(0)
	for zoomCounter := minZoom; zoomCounter <= maxZoom; zoomCounter++ {
		cV := math.Pow(float64(2), float64(18-zoomCounter))
		unitSize := cV * 256
		for _, rectCounter := range rectAreas {
			minX := int64(math.Floor((111320.7019*rectCounter.left + 0.02068) / unitSize))
			maxX := int64(math.Floor((111320.7019*rectCounter.right + 0.02068) / unitSize))
			minY := int64(math.Floor((137651.4674*rectCounter.top - 673284.9677) / unitSize))
			maxY := int64(math.Floor((137651.4674*rectCounter.bottom - 673284.9677) / unitSize))
			for xCounter := minX; xCounter <= maxX; xCounter++ {
				for yCounter := minY; yCounter <= maxY; yCounter++ {
					counter++
				}
			}
		}
	}

	atomic.StoreUint64(&instance.jobStatus.total, counter)

	startMsg := fmt.Sprintf("下载开始，共计%d个文件。", atomic.LoadUint64(&instance.jobStatus.total))
	instance.putMessage(startMsg)

	mapPropertiesList := make([]*MapProperties, 0, instance.listCapacity)
	instance.currentDownloadTimes = 0
	threadCounter := 0
	instance.errorList.InitSave(instance.currentDownloadTimes, jobPath)

	for zoomLevel := minZoom; zoomLevel <= maxZoom; zoomLevel++ {
		cV := math.Pow(float64(2), float64(18-zoomLevel))
		unitSize := cV * 256
		for _, rect := range rectAreas {
			minX := int64(math.Floor((111320.7019*rect.left + 0.02068) / unitSize))
			maxX := int64(math.Floor((111320.7019*rect.right + 0.02068) / unitSize))
			minY := int64(math.Floor((137651.4674*rect.top - 673284.9677) / unitSize))
			maxY := int64(math.Floor((137651.4674*rect.bottom - 673284.9677) / unitSize))
			for x := minX; x <= maxX; x++ {
				for y := minY; y <= maxY; y++ {
					if len(mapPropertiesList) >= instance.listCapacity {
						if threadCounter >= instance.threadCount {
							<-instance.channel
							threadCounter--
						}

						go instance.downloadMapBySlices(&jobPath, mapPropertiesList, instance.channel, &instance.jobStatus)
						threadCounter++

						mapPropertiesList = make([]*MapProperties, 0, instance.listCapacity)
					}
					mapProperties := &MapProperties{zoomLevel, x, y}
					mapPropertiesList = append(mapPropertiesList, mapProperties)
				}
			}
		}
	}

	if len(mapPropertiesList) > 0 {
		if threadCounter >= instance.threadCount {
			<-instance.channel
			threadCounter--
		}
		go instance.downloadMapBySlices(&jobPath, mapPropertiesList, instance.channel, &instance.jobStatus)
		threadCounter++
	}

	for i := 0; i < threadCounter; i++ {
		<-instance.channel
	}
	instance.errorList.CloseSave()

	instance.currentDownloadTimes++
	msg := fmt.Sprintf("第%d轮数据下载完成，共计%d个文件，%d个文件下载成功，%d个文件下载失败。", instance.currentDownloadTimes, atomic.LoadUint64(&instance.jobStatus.total), atomic.LoadUint64(&instance.jobStatus.counter), atomic.LoadUint64(&instance.jobStatus.errorCounter))
	instance.putMessage(msg)
}

func (instance *GetBaiduMap) fetchErrorList(jobPath string, total uint64) {
	instance.Init()
	atomic.StoreUint64(&instance.jobStatus.total, total)

	mapPropertiesList := make([]*MapProperties, 0, instance.listCapacity)
	threadCounter := 0

	instance.errorList.InitLoad(instance.currentDownloadTimes-1, jobPath)
	instance.errorList.InitSave(instance.currentDownloadTimes, jobPath)

	for {
		lines := instance.errorList.ReadLine()
		if lines == nil {
			break
		}
		for _, value := range lines {
			if len(mapPropertiesList) >= instance.listCapacity {
				if threadCounter >= instance.threadCount {
					<-instance.channel
					threadCounter--
				}
				go instance.downloadMapBySlices(&jobPath, mapPropertiesList, instance.channel, &instance.jobStatus)
				threadCounter++

				mapPropertiesList = make([]*MapProperties, 0, instance.listCapacity)
			}
			mapProperties := value
			mapPropertiesList = append(mapPropertiesList, mapProperties)
		}
	}

	if len(mapPropertiesList) > 0 {
		if threadCounter >= instance.threadCount {
			<-instance.channel
			threadCounter--
		}
		go instance.downloadMapBySlices(&jobPath, mapPropertiesList, instance.channel, &instance.jobStatus)
		threadCounter++
	}

	for i := 0; i < threadCounter; i++ {
		<-instance.channel
	}
	instance.errorList.CloseRead()
	instance.errorList.CloseSave()

	instance.currentDownloadTimes++
	msg := fmt.Sprintf("第%d轮数据下载完成，共计%d个文件，%d个文件下载成功，%d个文件下载失败。", instance.currentDownloadTimes, atomic.LoadUint64(&instance.jobStatus.total), atomic.LoadUint64(&instance.jobStatus.counter), atomic.LoadUint64(&instance.jobStatus.errorCounter))
	instance.putMessage(msg)
}

// createJobPath 定义
func (instance *GetBaiduMap) analysePara(message []byte) (*DownloadParaStruct, error) {
	var dat map[string]interface{}
	if err := json.Unmarshal(message, &dat); err != nil {
		fmt.Println(err.Error())
	}

	var minZoomLevel, maxZoomLevel, provinces string
	if dat["MinZoomLevel"] != nil {
		minZoomLevel = dat["MinZoomLevel"].(string)
	}
	if dat["MaxZoomLevel"] != nil {
		maxZoomLevel = dat["MaxZoomLevel"].(string)
	}
	if dat["Province"] != nil {
		provinces = dat["Province"].(string)
	}
	minZoom, err := strconv.Atoi(minZoomLevel)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	maxZoom, err := strconv.Atoi(maxZoomLevel)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	return &DownloadParaStruct{
		minZoomLevel: minZoom,
		maxZoomLevel: maxZoom,
		provinces:    provinces,
	}, nil
}

// download 定义
func (instance *GetBaiduMap) download(message []byte) {
	instance.setDownloadFlag(true)
	defer instance.setDownloadFlag(false)
	go instance.putProcessingMessage()

	para, err1 := instance.analysePara(message)
	if err1 != nil {
		fmt.Println(err1.Error())
		return
	}

	instance.currentDownloadTimes = 0

	jobPath, err2 := instance.createJobPath()
	if err2 != nil {
		fmt.Println(err2.Error())
		return
	}

	rectAreas := instance.getDownloadingAreas(para.provinces)

	instance.fetchMaps(jobPath, para.minZoomLevel, para.maxZoomLevel, rectAreas)

	for {
		if atomic.LoadUint64(&instance.jobStatus.errorCounter) == 0 {
			break
		}
		instance.fetchErrorList(jobPath, atomic.LoadUint64(&instance.jobStatus.errorCounter))
	}
}

// setDownloadFlag 定义
func (instance *GetBaiduMap) setDownloadFlag(flg bool) {
	instance.downloadFlag = flg
}

// putMessage 定义
func (instance *GetBaiduMap) putMessage(message string) {
	if instance.broadcastMessageCallback != nil {
		go instance.broadcastMessageCallback(message)
	}
}

// putProcessingMessage 定义
func (instance *GetBaiduMap) putProcessingMessage() {
	for {
		if instance.downloadFlag == false {
			break
		}
		if instance.jobStatus.counter > 0 {
			msg := fmt.Sprintf("正在进行第%d轮数据下载，%d个文件下载成功，共计%d个文件，%d个文件下载失败。", instance.currentDownloadTimes+1, atomic.LoadUint64(&instance.jobStatus.counter), atomic.LoadUint64(&instance.jobStatus.total), atomic.LoadUint64(&instance.jobStatus.errorCounter))
			instance.putMessage(msg)
			time.Sleep(3 * time.Second)
		}
	}
}

// Run 定义
func (instance *GetBaiduMap) Run(message []byte) {
	if instance.downloadFlag == true {
		return
	}
	go instance.download(message)
}
