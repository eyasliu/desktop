package desktop

import (
	_ "embed"
	"fmt"

	"github.com/eyasliu/desktop/tray"
)

//
type logger interface {
	Info(v ...interface{})
}

type defaultLogger struct{}

func (l *defaultLogger) Info(v ...interface{}) {
	fmt.Println(v...)
}

var _ logger = &defaultLogger{}

// Options 打开的窗口和系统托盘配置
type Options struct {
	// 系统托盘图片设置，可使用 IconPath 和 IconBytes 二选其一方式设置
	// 系统托盘图标路径，建议使用绝对路径，如果相对路径取执行文件相对位置
	IconPath string
	// 系统托盘图标文件内容
	IconBytes []byte
	// 是否启用调试模式，启用后webview支持右键菜单，并且支持打开 chrome 开发工具
	Debug bool
	// 启动webview后访问的url
	StartURL string
	// 当webview访问错误时显示的错误页面
	FallbackPage string
	// webview2 底层使用的用户数据目录
	DataPath string
	// webview 窗口默认显示的标题文字
	Title string
	// webview窗口宽度
	Width int
	// webview 窗口高度
	Height int
	// 刚启动webview时是否隐藏状态，可通过 Show() 方法显示
	StartHidden bool
	// 当用户关闭了webview窗口时是否视为隐藏窗口，如果为false，用户关闭窗口后会触发销毁应用
	HideWindowOnClose bool
	// webview 窗口是否总是在最顶层
	AlwaysOnTop bool
	// 系统托盘设置
	Tray *tray.Tray
	// 打印日志的实例
	Logger logger
	// 是否去掉webview窗口的边框，注意无边框会把右上角最大化最小化等按钮去掉
	Frameless bool
	// 打开窗口时是否自动在屏幕中间
	Center bool
	// 打开窗口时是否自动聚焦
	AutoFocus bool
}

//go:embed desktop.ico
var defaultTrayIcon []byte

// GetIcon 获取托盘图标路径
func (o *Options) GetIcon() string {
	if o.IconPath != "" {
		return o.IconPath
	} else if len(o.IconBytes) > 1 {
		iconpath, err := iconBytesToFilePath(o.IconBytes)
		if err == nil {
			return iconpath
		}
	}
	ip, _ := iconBytesToFilePath(defaultTrayIcon)
	return ip
}

// WebView 定义webview 功能操作，暴露给外部调用
type WebView interface {

	// Run 开始运行webview，调用后 webview 才会显示
	//
	//  注: 必须要和 New 在同一个 goroutine 执行
	Run()

	// Destroy 销毁当前的窗口
	Destroy()

	// SetTitle 设置窗口的标题文字
	SetTitle(title string)

	// Navigate webview窗口跳转到指定url
	Navigate(url string)

	// SetHtml webview窗口显示为指定的 html 内容
	SetHtml(html string)

	// Init 在页面初始化的时候注入的js代码，页面无论是跳转还是刷新后都会重新执行 Init 注入的代码
	// 触发的时机会在 window.onload 之前
	Init(js string)

	// Eval 在webview页面执行js代码
	Eval(js string)

	// Bind 注入JS函数，底层通过 Init 实现，用于往页面注入函数，实现 JS 和 Go 互相调用
	// name 是函数名，
	// fn 必须是 go 函数，否则无效，
	// 注入的函数调用后返回 Promise，在Promise resolve 获取go函数返回值，
	// 注入的函数参数个数必须和js调用时传入的参数类型和个数一致，否则 reject
	Bind(name string, f interface{})

	// Hide 隐藏窗口
	Hide()
	// Show 显示窗口
	Show()
}
