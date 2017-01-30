package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// DownloadErrorInfo 定义
type DownloadErrorInfo struct {
	listCaption           int
	writtingErrorFile     *os.File
	writtingErrorFileName string
	writtingErrorList     []*MapProperties
	readingErrorFile      *os.File
	reader                *bufio.Reader
	mu                    sync.Mutex
}

// InitSave 定义
func (errorMaps *DownloadErrorInfo) InitSave(downloadthreadCounter int, downloadPathName string) {
	errorMaps.mu.Lock()
	defer errorMaps.mu.Unlock()
	errorMaps.writtingErrorList = make([]*MapProperties, 0, errorMaps.listCaption)
	errorFileName := fmt.Sprintf("%s/errLst%d.err", downloadPathName, downloadthreadCounter)
	errorMaps.writtingErrorFileName = errorFileName
	// errorMaps.writtingErrorFile = nil
	// if errorMaps.writtingErrorFile == nil {
	var err error
	errorMaps.writtingErrorFile, err = os.OpenFile(errorMaps.writtingErrorFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		fmt.Println(err.Error())
		errorMaps.writtingErrorFile = nil
		errorMaps.writtingErrorList = nil
		return
	}
	// }
}

// InitLoad 定义
func (errorMaps *DownloadErrorInfo) InitLoad(downloadthreadCounter int, downloadPathName string) {
	errorMaps.mu.Lock()
	defer errorMaps.mu.Unlock()
	errorFileName := fmt.Sprintf("%s/errLst%d.err", downloadPathName, downloadthreadCounter)
	var err error
	errorMaps.readingErrorFile, err = os.Open(errorFileName)
	if err != nil {
		fmt.Println(err.Error())
		errorMaps.readingErrorFile = nil
		errorMaps.reader = nil
	}
	errorMaps.reader = bufio.NewReader(errorMaps.readingErrorFile)
}

// saveLog 定义
func (errorMaps *DownloadErrorInfo) saveLog() {
	if errorMaps.writtingErrorList != nil && len(errorMaps.writtingErrorList) != 0 {

		var buf bytes.Buffer
		for _, value := range errorMaps.writtingErrorList {
			message := fmt.Sprintf("%d,%d,%d\t", value.zoomLevel, value.x, value.y)
			buf.WriteString(message)
		}
		fmt.Fprintln(errorMaps.writtingErrorFile, buf.String())
	}
}

// Append 定义
func (errorMaps *DownloadErrorInfo) Append(mapProperties []*MapProperties) {
	errorMaps.mu.Lock()
	defer errorMaps.mu.Unlock()
	if errorMaps.writtingErrorList == nil {
		return
	}
	for _, value := range mapProperties {
		errorMaps.writtingErrorList = append(errorMaps.writtingErrorList, value)
		if len(errorMaps.writtingErrorList) >= errorMaps.listCaption {
			errorMaps.saveLog()
			errorMaps.writtingErrorList = make([]*MapProperties, 0, errorMaps.listCaption)
		}
	}
}

// ReadLine 定义
func (errorMaps *DownloadErrorInfo) ReadLine() (mapPropertiesList []*MapProperties) {
	errorMaps.mu.Lock()
	defer errorMaps.mu.Unlock()
	if errorMaps.readingErrorFile == nil {
		return
	}
	// var buf string
	// _,err := fmt.Fscanln(errorMaps.redingErrorFile,&buf)
	// _,err := fmt.Fscanf(errorMaps.readingErrorFile,"%s\n",&buf)
	buf, err := errorMaps.reader.ReadString('\n')

	if err == io.EOF {
		return
	} else if err != nil {
		fmt.Println(err.Error())
	}
	errorDatas := strings.Split(buf, "\t")
	mapPropertiesList = make([]*MapProperties, 0, errorMaps.listCaption)
	for _, value := range errorDatas {
		var zoomLevel int
		var x, y int64
		_, err = fmt.Sscanf(value, "%d,%d,%d", &zoomLevel, &x, &y)
		if err == nil {
			mapProperties := &MapProperties{zoomLevel, x, y}
			mapPropertiesList = append(mapPropertiesList, mapProperties)
		}
	}
	return
}

// CloseSave 定义
func (errorMaps *DownloadErrorInfo) CloseSave() {
	errorMaps.mu.Lock()
	defer errorMaps.mu.Unlock()
	errorMaps.saveLog()
	errorMaps.writtingErrorFile.Close()
	errorMaps.writtingErrorFileName = ""
	errorMaps.writtingErrorFile = nil
}

// CloseRead 定义
func (errorMaps *DownloadErrorInfo) CloseRead() {
	errorMaps.reader = nil
	errorMaps.readingErrorFile.Close()
	errorMaps.readingErrorFile = nil
}
