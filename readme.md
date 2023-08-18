# 桌面开发

一套可定制化的桌面开发工具，集成了 webview 窗口、系统托盘的机制

## 特性

- 基于 webview 的桌面开发工具，使用 webview2 驱动，纯 Go 语言实现，无 CGO 依赖
- 启动窗口时自动检测 webview2 环境，如果未安装，则自动运行安装 webview2 引导
- 支持 webview 常规操作，如跳转，注入 js，js 与 go 交互等操作
- 支持 css 设置 `-webkit-app-region: drag` 后拖拽窗口
- 系统托盘支持，托盘支持菜单，支持无限级子菜单
- TODO: 自更新机制

# DEMO


https://github.com/eyasliu/desktop/assets/4774683/2c21e38a-cead-47d8-b847-01345b77329d

可使用以下命令快速启动示例程序

```
go run github.com/eyasliu/desktop/example/basic
```

demo 源码

```go
func main() {
  var app desktop.WebView
  app = desktop.New(&desktop.Options{
    Debug:             true,
    AutoFocus:         true,
    Width:             1280,
    Height:            768,
    HideWindowOnClose: true,
    StartURL:          "https://www.wps.cn",
    Tray: &tray.Tray{
      Title:   "托盘演示",
      Tooltip: "提示文字，点击激活显示窗口",
      OnClick: func() {
        app.Show() // 显示窗口
      },
      Items: []*tray.TrayItem{
        {
          Title:   "跳转到 wps 365 官网",
          OnClick: func() { app.Navigate("https://365.wps.cn") },
        },
        {
          Title:   "打开本地页面",
          OnClick: func() { app.SetHtml("<h1>这是个本地页面</h1>") },
        },
        {
          Title:   "执行js alert('hello')",
          OnClick: func() { app.Eval("alert('hello')") },
        },

        {
          Title: "窗口操作",
          Items: []*tray.TrayItem{
            {
              Title: "新窗口打开 WPS AI",
              OnClick: func() {
                go func() {
                  wpsai := desktop.New(&desktop.Options{
                    StartURL: "https://ai.wps.cn",
                    Frameless:         true, // 去掉边框
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
    },
  })

  app.Run()
}

```

# API

[点击查看文档](https://pkg.go.dev/github.com/eyasliu/desktop)

# 注意事项

windows 的桌面开发对于操作线程

- desktop.New 和 app.Run 必须要在同一个 goroutine 协程执行
- 一个 goroutine 只能 Run 一个 desktop.WebView
