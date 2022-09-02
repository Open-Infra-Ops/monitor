package PrometheusClient

import (
	"encoding/json"
	"fmt"
	"github.com/astaxie/beego/config"
	"github.com/astaxie/beego/logs"
)

func InitLogger(BConfig config.Configer) (err error) {
	logConf := make(map[string]interface{})
	maxLines, lErr := BConfig.Int64("log::maxlines")
	if lErr != nil {
		maxLines = 20000
	}
	logConf["maxlines"] = maxLines
	logConf["filename"] = BConfig.String("log::log_path")
	level, _ := BConfig.Int("log::log_level")
	logConf["level"] = level
	confStr, err := json.Marshal(logConf)
	if err != nil {
		fmt.Println("marshal failed,err:", err.Error())
		return
	}
	err = logs.SetLogger(logs.AdapterFile, string(confStr))
	if err != nil {
		fmt.Println("marshal failed,err:", err.Error())
		return
	}
	logs.SetLogFuncCall(true)
	return
}

func LogInit(c config.Configer) {
	err := InitLogger(c)
	if err != nil {
		fmt.Println(err)
	}
	logs.Info("log init success!")
}
