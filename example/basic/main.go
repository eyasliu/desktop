package main

import (
	_ "embed"
	"fmt"
	"runtime"

	"github.com/eyasliu/desktop"
	"github.com/eyasliu/desktop/tray"
)

//go:embed dog.ico
var dogIco []byte

func main() {
	var app desktop.WebView
	var appTray *tray.Tray
	checkedMenu := &tray.TrayItem{
		Title:    "有勾选状态菜单项",
		Checkbox: true,
		Checked:  true,
	}
	checkedMenu.OnClick = func() {
		checkedMenu.Checked = !checkedMenu.Checked
		checkedMenu.Title = "未勾选"
		if checkedMenu.Checked {
			checkedMenu.Title = "已勾选"
		}
		checkedMenu.Update()
	}
	appTray = &tray.Tray{
		Title:   "托盘演示",
		Tooltip: "提示文字，点击激活显示窗口",
		OnClick: func() {
			app.Show() // 显示窗口
		},
		Items: []*tray.TrayItem{
			checkedMenu,
			{
				Title: "修改托盘图标和文字",
				OnClick: func() {
					appTray.SetIconBytes(dogIco)
					appTray.SetTooltip("这是设置过后的托盘提示文字")
				},
			},
			{
				Title:   "跳转到腾讯文档",
				OnClick: func() { app.Navigate("https://docs.qq.com") },
			},
			{
				Title: "打开本地页面",
				OnClick: func() {
					app.SetHtml(`<h1>这是个本地页面</h1>
				<div style="-webkit-app-region: drag">设置css： -webkit-app-region: drag 可移动窗口</div>`)
				},
			},
			{
				Title: "JS 交互",
				Items: []*tray.TrayItem{
					{
						Title:   "执行 alert('hello')",
						OnClick: func() { app.Eval("alert('hello')") },
					},
					{
						Title: "每次进入页面执行alert",
						OnClick: func() {
							app.Init("alert('每次进入页面都会执行一次')")
						},
					},
					{
						Title: "调用Go函数",
						OnClick: func() {

							app.Eval(`golangFn('tom').then(s => alert(s))`)
						},
					},
				},
			},
			{
				Title: "窗口操作",
				Items: []*tray.TrayItem{
					{
						Title: "无边框打开新窗口 im.qq.com",
						OnClick: func() {
							go func() {
								wpsai := desktop.New(&desktop.Options{
									StartURL:  "https://im.qq.com",
									Center:    true,
									Frameless: true, // 去掉边框
								})
								wpsai.Run()
							}()
						},
					},
					{
						Title: "显示窗口",
						OnClick: func() {
							app.Show()
						},
					},
					{
						Title: "隐藏窗口",
						OnClick: func() {
							app.Hide()
						},
					},
					{
						Title: "设置窗口标题",
						OnClick: func() {
							app.SetTitle("这是新的标题")
						},
					},
				},
			},
			{
				Title: "退出程序",
				OnClick: func() {
					app.Destroy()
				},
			},
		},
	}
	app = desktop.New(&desktop.Options{
		Debug:             true,
		AutoFocus:         true,
		Width:             1280,
		Height:            768,
		HideWindowOnClose: true,
		Center:            true,
		Title:             "basic 演示",
		StartURL:          "https://www.wps.cn",
		Tray:              appTray,
	})

	app.Bind("golangFn", func(name string) string {
		return fmt.Sprintf(`Hello %s, GOOS=%s`, name, runtime.GOOS)
	})

	app.Run()
}
