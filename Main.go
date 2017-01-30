package main

import (
)

func main() {
    config := NewConfig()
    
    webSocketService := NewWebSocketService("web","home.html",config.Port)
    getBaiduMap := NewGetBaiduMap(config,webSocketService.BroadcastMessage)
    webSocketService.submitCallback = getBaiduMap.Run
    
    webSocketService.Start()
}