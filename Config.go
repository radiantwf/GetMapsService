package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// ConfigJSONStruct 定义
type ConfigJSONStruct struct {
	Version                  string
	UpdateDate               string
	Port                     int
	AllowedThreadCount       int
	ProcessListCapacity      int
	ProcessErrorListCapacity int
	ProvinceInformation      []map[string]interface{}
}

// ConfigStruct 定义
type ConfigStruct struct {
	Version                  string
	UpdateDate               string
	Port                     int
	AllowedThreadCount       int
	ProcessListCapacity      int
	ProcessErrorListCapacity int
	ProvinceInformation      []ProvinceInfoStruct
}

// ProvinceInfoStruct 定义
type ProvinceInfoStruct struct {
	province string
	area     AreaStruct
}

// AreaStruct 定义
type AreaStruct struct {
	longitude [2]float64
	latitude  [2]float64
}

// NewConfig 定义
func NewConfig() *ConfigStruct {
	config := new(ConfigStruct)
	var jsonStruct ConfigJSONStruct
	err := config.loadJSONFile(&jsonStruct)
	if err != nil {
		return nil
	}

	config.Version = jsonStruct.Version
	config.UpdateDate = jsonStruct.UpdateDate
	config.Port = jsonStruct.Port
	config.AllowedThreadCount = jsonStruct.AllowedThreadCount
	config.ProcessListCapacity = jsonStruct.ProcessListCapacity
	config.ProcessErrorListCapacity = jsonStruct.ProcessErrorListCapacity

	// if runtime.GOOS == "darwin" {
	// }
	// if config.AllowedThreadCount > 100 {
	// 	config.AllowedThreadCount = 100
	// }

	config.ProvinceInformation = make([]ProvinceInfoStruct, 0, 50)
	for _, value := range jsonStruct.ProvinceInformation {
		var p ProvinceInfoStruct
		p.province = value["province"].(string)
		longitudeInterface := value["area"].(map[string]interface{})["longitude"].([]interface{})
		latitudeInterface := value["area"].(map[string]interface{})["latitude"].([]interface{})
		p.area = AreaStruct{[2]float64{longitudeInterface[0].(float64), longitudeInterface[1].(float64)}, [2]float64{latitudeInterface[0].(float64), latitudeInterface[1].(float64)}}
		config.ProvinceInformation = append(config.ProvinceInformation, p)
	}
	fmt.Println("当前配置文件信息为：")
	fmt.Println(config)
	return config
}

// loadJSONFile 定义
func (config *ConfigStruct) loadJSONFile(jsonStruct *ConfigJSONStruct) (err error) {
	var jsonStr []byte
	jsonStr, err = ioutil.ReadFile("config/config.json")
	if err != nil {
		return
	}
	err = json.Unmarshal(jsonStr, jsonStruct)
	return
}
