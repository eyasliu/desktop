package webview2

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/eyasliu/desktop/go-webview2/internal/w32"
	"github.com/eyasliu/desktop/tray"
)

type eventName byte

const (
	eventInit eventName = iota
	eventTerminate
	eventDispatch
	eventDestroy
	eventSetTitle
	eventSetSize
	eventNavigate
	eventSetHtml
	eventEval
	eventBind
	eventHide
	eventShow
)

type winEvent struct {
	name eventName
	data any
}

type setSizeParam struct {
	width  int
	height int
	hint   Hint
}

type bindParam struct {
	name string
	fn   any
}

type Window struct {
	webview  *webview
	ready    bool
	readyMu  sync.Mutex
	preReady []func()
	hasTray  bool
}

func NewWin(option WebViewOptions, trayOpt *tray.Tray) *Window {
	runtime.LockOSThread()
	win := &Window{
		hasTray: trayOpt != nil,
	}
	win.webview = NewWithOptions(option).(*webview)

	return win
}

// 启动事件循环
func (w *Window) Run() {
	w.preRun()
	var msg w32.Msg
	for {
		_, _, _ = w32.User32GetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
		)
		if msg.Message == w32.WMApp {
			w.webview.m.Lock()
			q := append([]func(){}, w.webview.dispatchq...)
			w.webview.dispatchq = []func(){}
			w.webview.m.Unlock()
			for _, v := range q {
				v()
			}
		} else if msg.Message == w32.WMQuit {
			break
		} else if msg.Message == 10086 {
			event := (*winEvent)(unsafe.Pointer(msg.WParam))
			switch event.name {
			case eventInit:
				js := event.data.(string)
				w.webview.Init(js)
			case eventTerminate:
				w.webview.Terminate()
			case eventDispatch:
				fn := event.data.(func())
				w.webview.Dispatch(fn)
			case eventDestroy:
				if w.hasTray {
					tray.Quit()
				}
				w.webview.Destroy()
			case eventSetTitle:
				title := event.data.(string)
				w.webview.SetTitle(title)
			case eventSetSize:
				size := event.data.(setSizeParam)
				w.webview.SetSize(size.width, size.height, size.hint)
			case eventNavigate:
				u := event.data.(string)
				w.webview.Navigate(u)
			case eventSetHtml:
				html := event.data.(string)
				w.webview.SetHtml(html)
			case eventEval:
				js := event.data.(string)
				w.webview.Eval(js)
			case eventBind:
				b := event.data.(bindParam)
				w.webview.Bind(b.name, b.fn)
			case eventHide:
				w.webview.Hide()
			case eventShow:
				w.webview.Show()
			}
			continue
		}
		r, _, _ := w32.User32GetAncestor.Call(uintptr(msg.Hwnd), w32.GARoot)
		r, _, _ = w32.User32IsDialogMessage.Call(r, uintptr(unsafe.Pointer(&msg)))
		if r != 0 {
			continue
		}
		_, _, _ = w32.User32TranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = w32.User32DispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	runtime.UnlockOSThread()
}

func (w *Window) onReady() {
	if w.ready {
		return
	}
	w.readyMu.Lock()
	w.ready = true
	q := append([]func(){}, w.preReady...)
	w.preReady = nil // 用完后就没用了
	w.readyMu.Unlock()
	for _, v := range q {
		v()
	}
}

// 为了触发 w.onReady
func (w *Window) preRun() {
	w.webview.Bind("__webview_onready", w.onReady)
	w.webview.Init(`window.addEventListener("load", function(){__webview_onready()});`)
}

// 当 webview 启动了，但是没有完全启动好，就 postmessage 的话，会导致初始化异常，导致奇怪bug
// 所以用 w.ready 之前的消息先存起来，等准备好了之后一起发送
func (w *Window) dispatch(name eventName, data any) {
	if !w.ready {
		w.readyMu.Lock()
		w.preReady = append(w.preReady, func() {
			w.dispatch(name, data)
		})
		w.readyMu.Unlock()
		return
	}
	w32.User32PostThreadMessageW.Call(
		w.webview.mainthread,
		10086,
		uintptr(unsafe.Pointer(&winEvent{name, data})),
		0,
	)
}

func (w *Window) Terminate() {
	w.dispatch(eventTerminate, nil)
}
func (w *Window) Dispatch(f func()) {
	w.dispatch(eventDispatch, f)
}
func (w *Window) Destroy() {
	w.dispatch(eventDestroy, nil)
}
func (w *Window) Window() unsafe.Pointer {
	return w.webview.Window()
}
func (w *Window) SetTitle(title string) {
	w.dispatch(eventSetTitle, title)
}
func (w *Window) SetSize(width int, height int, hint Hint) {
	w.dispatch(eventSetSize, setSizeParam{width, height, hint})
}

func (w *Window) Init(js string) {
	w.dispatch(eventInit, js)
}
func (w *Window) Bind(name string, f interface{}) {
	w.dispatch(eventBind, bindParam{name, f})
}

func (w *Window) Navigate(url string) {
	w.dispatch(eventNavigate, url)
}

func (w *Window) SetHtml(html string) {
	w.dispatch(eventSetHtml, html)
}

func (w *Window) Eval(js string) {
	w.dispatch(eventEval, js)
}

func (w *Window) Show() {
	w.dispatch(eventShow, nil)
}

func (w *Window) Hide() {
	w.dispatch(eventHide, nil)
}
