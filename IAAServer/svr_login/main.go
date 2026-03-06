package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// 配置项：小游戏AppID和AppSecret（请替换为实际值）
const (
	WXAppID     = "wx735dd8b2e0fda8fe"
	WXAppSecret = "ed909b959713aef0ce156f3131175ead"
	ServerPort  = ":80" // 监听80端口
)

// 微信接口返回的结构体
type WxCode2SessionResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}
type TestResp struct {
	TestRespContentHeader string `json:"content"`
}

// 给Unity返回的结构体
type LoginResp struct {
	OpenID string `json:"openid"`
	ErrMsg string `json:"errMsg"`
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	// 设置跨域头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json;charset=utf-8")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(TestResp{TestRespContentHeader: r.Method})
	return
}

// 核心登录接口：/wxlogin
func wxLoginHandler(w http.ResponseWriter, r *http.Request) {
	// 设置跨域头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json;charset=utf-8")

	// 处理OPTIONS预检请求
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 只接受POST请求
	if r.Method != "POST" {
		json.NewEncoder(w).Encode(LoginResp{ErrMsg: "仅支持POST请求"})
		return
	}

	// 解析表单参数
	if err := r.ParseForm(); err != nil {
		json.NewEncoder(w).Encode(LoginResp{ErrMsg: "解析参数失败：" + err.Error()})
		return
	}

	code := r.PostForm.Get("code")
	appid := r.PostForm.Get("appid")

	// 校验参数
	if code == "" {
		json.NewEncoder(w).Encode(LoginResp{ErrMsg: "code不能为空"})
		return
	}
	if appid != WXAppID {
		json.NewEncoder(w).Encode(LoginResp{ErrMsg: fmt.Sprintf("AppID错误: incoming app id -> %s", appid)})
		return
	}

	// 调用微信接口换取openid
	wxUrl := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		url.QueryEscape(WXAppID),
		url.QueryEscape(WXAppSecret),
		url.QueryEscape(code),
	)

	wxResp, err := http.Get(wxUrl)
	if err != nil {
		json.NewEncoder(w).Encode(LoginResp{ErrMsg: "调用微信接口失败：" + err.Error()})
		return
	}
	defer wxResp.Body.Close()

	// 解析微信返回的JSON
	var wxResult WxCode2SessionResp
	if err := json.NewDecoder(wxResp.Body).Decode(&wxResult); err != nil {
		json.NewEncoder(w).Encode(LoginResp{ErrMsg: "解析微信返回数据失败：" + err.Error()})
		return
	}

	// 处理微信接口业务错误
	if wxResult.ErrCode != 0 {
		json.NewEncoder(w).Encode(LoginResp{ErrMsg: fmt.Sprintf("微信接口错误：%d - %s", wxResult.ErrCode, wxResult.ErrMsg)})
		return
	}

	// 成功返回openid
	json.NewEncoder(w).Encode(LoginResp{
		OpenID: wxResult.OpenID,
		ErrMsg: "",
	})
}

func main() {
	http.HandleFunc("/wxlogin", wxLoginHandler)
	http.HandleFunc("/test", testHandler)
	fmt.Printf("服务器启动成功！监听地址：http://0.0.0.0%s\n", ServerPort)
	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		fmt.Printf("服务器启动失败：%s\n", err.Error())
	}
}
