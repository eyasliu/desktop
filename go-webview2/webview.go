//go:build windows
// +build windows

package webview2

import (
	"encoding/json"
	"errors"
	"log"
	"reflect"
	"strconv"
	"sync"
	"unsafe"

	"github.com/eyasliu/desktop/go-webview2/internal/w32"
	"github.com/eyasliu/desktop/go-webview2/pkg/edge"

	"golang.org/x/sys/windows"
)

var (
	windowContext     = map[uintptr]interface{}{}
	windowContextSync sync.RWMutex
)

func getWindowContext(wnd uintptr) interface{} {
	windowContextSync.RLock()
	defer windowContextSync.RUnlock()
	return windowContext[wnd]
}

func setWindowContext(wnd uintptr, data interface{}) {
	windowContextSync.Lock()
	defer windowContextSync.Unlock()
	windowContext[wnd] = data
}

// Loads an image from file to be shown in tray or menu item.
// LoadImage: https://msdn.microsoft.com/en-us/library/windows/desktop/ms648045(v=vs.85).aspx
func loadIconFrom(src string) (windows.Handle, error) {
	const IMAGE_ICON = 1               // Loads an icon
	const LR_LOADFROMFILE = 0x00000010 // Loads the stand-alone image from the file
	const LR_DEFAULTSIZE = 0x00000040  // Loads default-size icon for windows(SM_CXICON x SM_CYICON) if cx, cy are set to zero

	// Save and reuse handles of loaded images
	// t.muLoadedImages.RLock()
	// h, ok := t.loadedImages[src]
	// t.muLoadedImages.RUnlock()
	// if !ok {
	srcPtr, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return 0, err
	}
	res, _, err := w32.User32LoadImageW.Call(
		0,
		uintptr(unsafe.Pointer(srcPtr)),
		IMAGE_ICON,
		0,
		0,
		LR_LOADFROMFILE|LR_DEFAULTSIZE,
	)
	if res == 0 {
		return 0, err
	}
	h := windows.Handle(res)
	// t.muLoadedImages.Lock()
	// t.loadedImages[src] = h
	// t.muLoadedImages.Unlock()
	// }
	return h, nil
}

type navigationCompletedArg struct {
	Success bool
}

type browser interface {
	Embed(hwnd uintptr) bool
	Resize()
	Navigate(url string) error
	NavigateToString(htmlContent string)
	Init(script string)
	Eval(script string)
	NotifyParentWindowPositionChanged() error
	Focus()
}

type webview struct {
	hwnd        uintptr
	mainthread  uintptr
	browser     browser
	autofocus   bool
	hideOnClose bool
	maxsz       w32.Point
	minsz       w32.Point
	m           sync.Mutex
	bindings    map[string]interface{}
	dispatchq   []func()
	logger      logger
}

type logger interface {
	Info(v ...interface{})
}

type WindowOptions struct {
	Title     string
	Width     uint
	Height    uint
	IconId    uint
	Icon      string
	Center    bool
	Frameless bool
}

type WebViewOptions struct {
	Window            unsafe.Pointer
	StartURL          string
	FallbackPage      string
	StartHTML         string
	HideWindowOnClose bool

	// if true, enable context menu and chrome devtools
	Debug bool

	// DataPath specifies the datapath for the WebView2 runtime to use for the
	// browser instance.
	DataPath string

	// AutoFocus will try to keep the WebView2 widget focused when the window
	// is focused.
	AutoFocus bool

	Logger logger

	// WindowOptions customizes the window that is created to embed the
	// WebView2 widget.
	WindowOptions WindowOptions
}

// New creates a new webview in a new window.
func New(debug bool) WebView { return NewWithOptions(WebViewOptions{Debug: debug}) }

// NewWindow creates a new webview using an existing window.
//
// Deprecated: Use NewWithOptions.
func NewWindow(debug bool, window unsafe.Pointer) WebView {
	return NewWithOptions(WebViewOptions{Debug: debug, Window: window})
}

// NewWithOptions creates a new webview using the provided options.
func NewWithOptions(options WebViewOptions) WebView {
	w := &webview{}
	w.logger = options.Logger
	w.bindings = map[string]interface{}{}
	w.autofocus = options.AutoFocus
	w.hideOnClose = options.HideWindowOnClose

	chromium := edge.NewChromium()
	chromium.MessageCallback = w.msgcb
	chromium.DataPath = options.DataPath
	chromium.SetPermission(edge.CoreWebView2PermissionKindClipboardRead, edge.CoreWebView2PermissionStateAllow)

	if ok := chromium.CheckOrInstallWv2(); !ok {
		return nil
	}

	w.browser = chromium
	w.mainthread, _, _ = w32.Kernel32GetCurrentThreadID.Call()
	if !w.CreateWithOptions(options.WindowOptions) {
		return nil
	}

	settings, err := chromium.GetSettings()
	if err != nil {
		log.Fatal(err)
	}

	if !options.Debug {
		// disable context menu
		err = settings.PutAreDefaultContextMenusEnabled(options.Debug)
		if err != nil {
			log.Fatal(err)
		}

		// disable developer tools
		err = settings.PutAreDevToolsEnabled(options.Debug)
		if err != nil {
			log.Fatal(err)
		}
	}

	if options.FallbackPage != "" {
		w.SetFallbackPage(options.FallbackPage)
	}

	if options.StartURL != "" {
		w.Navigate(options.StartURL)
	} else if options.StartHTML != "" {
		w.SetHtml(options.StartHTML)
	}

	return w
}

type rpcMessage struct {
	ID     int               `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func jsString(v interface{}) string { b, _ := json.Marshal(v); return string(b) }

func (w *webview) msgcb(msg string) {
	d := rpcMessage{}
	if err := json.Unmarshal([]byte(msg), &d); err != nil {
		log.Printf("invalid RPC message: %v", err)
		return
	}

	id := strconv.Itoa(d.ID)
	if res, err := w.callbinding(d); err != nil {
		w.Dispatch(func() {
			w.Eval("window._rpc[" + id + "].reject(" + jsString(err.Error()) + "); window._rpc[" + id + "] = undefined")
		})
	} else if b, err := json.Marshal(res); err != nil {
		w.Dispatch(func() {
			w.Eval("window._rpc[" + id + "].reject(" + jsString(err.Error()) + "); window._rpc[" + id + "] = undefined")
		})
	} else {
		w.Dispatch(func() {
			w.Eval("window._rpc[" + id + "].resolve(" + string(b) + "); window._rpc[" + id + "] = undefined")
		})
	}
}

func (w *webview) callbinding(d rpcMessage) (interface{}, error) {
	w.m.Lock()
	f, ok := w.bindings[d.Method]
	w.m.Unlock()
	if !ok {
		return nil, nil
	}

	v := reflect.ValueOf(f)
	isVariadic := v.Type().IsVariadic()
	numIn := v.Type().NumIn()
	if (isVariadic && len(d.Params) < numIn-1) || (!isVariadic && len(d.Params) != numIn) {
		return nil, errors.New("function arguments mismatch")
	}
	args := []reflect.Value{}
	for i := range d.Params {
		var arg reflect.Value
		if isVariadic && i >= numIn-1 {
			arg = reflect.New(v.Type().In(numIn - 1).Elem())
		} else {
			arg = reflect.New(v.Type().In(i))
		}
		if err := json.Unmarshal(d.Params[i], arg.Interface()); err != nil {
			return nil, err
		}
		args = append(args, arg.Elem())
	}

	errorType := reflect.TypeOf((*error)(nil)).Elem()
	res := v.Call(args)
	switch len(res) {
	case 0:
		// No results from the function, just return nil
		return nil, nil

	case 1:
		// One result may be a value, or an error
		if res[0].Type().Implements(errorType) {
			if res[0].Interface() != nil {
				return nil, res[0].Interface().(error)
			}
			return nil, nil
		}
		return res[0].Interface(), nil

	case 2:
		// Two results: first one is value, second is error
		if !res[1].Type().Implements(errorType) {
			return nil, errors.New("second return value must be an error")
		}
		if res[1].Interface() == nil {
			return res[0].Interface(), nil
		}
		return res[0].Interface(), res[1].Interface().(error)

	default:
		return nil, errors.New("unexpected number of return values")
	}
}

// 实现通过 css 样式定义 -webkit-app-region: drag 可拖动窗口
func (w *webview) appRegion() {
	w.Init(`window.addEventListener('DOMContentLoaded', () => {
    document.body.addEventListener('mousedown', evt => {
        const { target } = evt;
        const appRegion = getComputedStyle(target)['-webkit-app-region'];

        if (appRegion === 'drag') {
            window.__drapAppRegion();
            evt.preventDefault();
            evt.stopPropagation();
        }
    });
});`)
	w.Bind("__drapAppRegion", w.dragAppRegion)
}

func (w *webview) dragAppRegion() {
	w32.User32ReleaseCapture.Call(w.hwnd)
	w32.User32PostMessageW.Call(w.hwnd, 161, 2, 0)
}

func (w *webview) updateWinForDpi(hwnd uintptr) {
	var bounds w32.Rect
	w32.User32GetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&bounds)))

	if bounds.Right == 0 || bounds.Bottom == 0 {
		return
	}

	posX := bounds.Left
	posY := bounds.Top
	width := bounds.Right - bounds.Left
	height := bounds.Bottom - bounds.Top

	_iDpi, _, _ := w32.User32GetDpiForWindow.Call(hwnd)
	iDpi := int32(_iDpi)
	dpiScaledX := uintptr((posX * iDpi) / 960)
	dpiScaledY := uintptr((posY * iDpi) / 960)

	dpiScaledWidth := uintptr((width * iDpi) / 96)
	dpiScaledHeight := uintptr((height * iDpi) / 96)
	w32.User32SetWindowPos.Call(hwnd, 0, dpiScaledX, dpiScaledY, dpiScaledWidth, dpiScaledHeight, 0x0010|0x0004)
}

func (w *webview) wndproc(hwnd, msg, wp, lp uintptr) uintptr {
	if w, ok := getWindowContext(hwnd).(*webview); ok {
		switch msg {
		case w32.WMCreate, w32.WMDpiChanged:
			w.updateWinForDpi(hwnd)
		case w32.WMMove, w32.WMMoving:
			_ = w.browser.NotifyParentWindowPositionChanged()
		case w32.WMNCLButtonDown:
			_, _, _ = w32.User32SetFocus.Call(w.hwnd)
			r, _, _ := w32.User32DefWindowProcW.Call(hwnd, msg, wp, lp)
			return r
		case w32.WMSize:
			w.browser.Resize()
		case w32.WMActivate:
			if wp == w32.WAInactive {
				break
			}
			if w.autofocus {
				w.browser.Focus()
			}
		case w32.WMClose:
			if w.hideOnClose {
				w.Hide()
			} else {
				_, _, _ = w32.User32DestroyWindow.Call(hwnd)
			}
		case w32.WMDestroy:
			w.Terminate()
		case w32.WMGetMinMaxInfo:
			lpmmi := (*w32.MinMaxInfo)(unsafe.Pointer(lp))
			if w.maxsz.X > 0 && w.maxsz.Y > 0 {
				lpmmi.PtMaxSize = w.maxsz
				lpmmi.PtMaxTrackSize = w.maxsz
			}
			if w.minsz.X > 0 && w.minsz.Y > 0 {
				lpmmi.PtMinTrackSize = w.minsz
			}
		default:
			r, _, _ := w32.User32DefWindowProcW.Call(hwnd, msg, wp, lp)
			return r
		}
		return 0
	}
	r, _, _ := w32.User32DefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func (w *webview) CreateWithOptions(opts WindowOptions) bool {
	_, _, _ = w32.ShcoreSetProcessDpiAwareness.Call(2)
	var hinstance windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &hinstance)

	var icon uintptr
	if len(opts.Icon) > 0 {
		hicon, err := loadIconFrom(opts.Icon)
		println(err)
		icon = uintptr(hicon)
	} else if opts.IconId == 0 {
		// load default icon
		icow, _, _ := w32.User32GetSystemMetrics.Call(w32.SystemMetricsCxIcon)
		icoh, _, _ := w32.User32GetSystemMetrics.Call(w32.SystemMetricsCyIcon)
		icon, _, _ = w32.User32LoadImageW.Call(uintptr(hinstance), 32512, icow, icoh, 0)
	} else {
		// load icon from resource
		icon, _, _ = w32.User32LoadImageW.Call(uintptr(hinstance), uintptr(opts.IconId), 1, 0, 0, w32.LR_DEFAULTSIZE|w32.LR_SHARED)
	}

	className, _ := windows.UTF16PtrFromString("webview")
	wc := w32.WndClassExW{
		CbSize:        uint32(unsafe.Sizeof(w32.WndClassExW{})),
		HInstance:     hinstance,
		LpszClassName: className,
		HIcon:         windows.Handle(icon),
		HIconSm:       windows.Handle(icon),
		LpfnWndProc:   windows.NewCallback(w.wndproc),
	}
	_, _, _ = w32.User32RegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	windowName, _ := windows.UTF16PtrFromString(opts.Title)

	windowWidth := opts.Width
	if windowWidth == 0 {
		windowWidth = 640
	}
	windowHeight := opts.Height
	if windowHeight == 0 {
		windowHeight = 480
	}

	var posX, posY uint
	if opts.Center {
		// get screen size
		screenWidth, _, _ := w32.User32GetSystemMetrics.Call(w32.SM_CXSCREEN)
		screenHeight, _, _ := w32.User32GetSystemMetrics.Call(w32.SM_CYSCREEN)
		// calculate window position
		posX = (uint(screenWidth) - windowWidth) / 2
		posY = (uint(screenHeight) - windowHeight) / 2
	} else {
		// use default position
		posX = w32.CW_USEDEFAULT
		posY = w32.CW_USEDEFAULT
	}

	var winSetting uintptr = w32.WSOverlappedWindow
	if opts.Frameless {
		winSetting = w32.WSPopupWindow | w32.WSMinimizeBox | w32.WSMaximizeBox | w32.WSSizeBox
	}

	w.hwnd, _, _ = w32.User32CreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		winSetting, // 0xCF0000, // WS_OVERLAPPEDWINDOW
		uintptr(posX),
		uintptr(posY),
		uintptr(windowWidth),
		uintptr(windowHeight),
		0,
		0,
		uintptr(hinstance),
		0,
	)
	setWindowContext(w.hwnd, w)
	w.updateWinForDpi(w.hwnd)

	_, _, _ = w32.User32ShowWindow.Call(w.hwnd, w32.SWShow)
	_, _, _ = w32.User32UpdateWindow.Call(w.hwnd)
	_, _, _ = w32.User32SetFocus.Call(w.hwnd)

	if !w.browser.Embed(w.hwnd) {
		return false
	}
	w.browser.Resize()

	w.appRegion()
	return true
}

func (w *webview) Destroy() {
	_, _, _ = w32.User32PostMessageW.Call(w.hwnd, w32.WMClose, 0, 0)
}

func (w *webview) Run() {}

func (w *webview) Terminate() {
	_, _, _ = w32.User32PostQuitMessage.Call(0)
}

func (w *webview) Window() unsafe.Pointer {
	return unsafe.Pointer(w.hwnd)
}

func (w *webview) Navigate(url string) {
	err := w.browser.Navigate(url)
	w.logger.Info("browser.Navigate:"+url+" ", err)
}

func (w *webview) SetHtml(html string) {
	w.browser.NavigateToString(html)
}

func (w *webview) SetTitle(title string) {
	_title, err := windows.UTF16FromString(title)
	if err != nil {
		_title, _ = windows.UTF16FromString("")
	}
	_, _, _ = w32.User32SetWindowTextW.Call(w.hwnd, uintptr(unsafe.Pointer(&_title[0])))
}

func (w *webview) SetSize(width int, height int, hints Hint) {
	index := w32.GWLStyle
	style, _, _ := w32.User32GetWindowLongPtrW.Call(w.hwnd, uintptr(index))
	if hints == HintFixed {
		style &^= (w32.WSThickFrame | w32.WSMaximizeBox)
	} else {
		style |= (w32.WSThickFrame | w32.WSMaximizeBox)
	}
	_, _, _ = w32.User32SetWindowLongPtrW.Call(w.hwnd, uintptr(index), style)

	if hints == HintMax {
		w.maxsz.X = int32(width)
		w.maxsz.Y = int32(height)
	} else if hints == HintMin {
		w.minsz.X = int32(width)
		w.minsz.Y = int32(height)
	} else {
		r := w32.Rect{}
		r.Left = 0
		r.Top = 0
		r.Right = int32(width)
		r.Bottom = int32(height)
		_, _, _ = w32.User32AdjustWindowRect.Call(uintptr(unsafe.Pointer(&r)), w32.WSOverlappedWindow, 0)
		_, _, _ = w32.User32SetWindowPos.Call(
			w.hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top),
			w32.SWPNoZOrder|w32.SWPNoActivate|w32.SWPNoMove|w32.SWPFrameChanged)
		w.browser.Resize()
	}
}

func (w *webview) Init(js string) {
	w.browser.Init(js)
}

func (w *webview) Eval(js string) {
	w.browser.Eval(js)
}

func (w *webview) Dispatch(f func()) {
	w.m.Lock()
	w.dispatchq = append(w.dispatchq, f)
	w.m.Unlock()
	_, _, _ = w32.User32PostThreadMessageW.Call(w.mainthread, w32.WMApp, 0, 0)
}

func (w *webview) Bind(name string, f interface{}) error {
	v := reflect.ValueOf(f)
	if v.Kind() != reflect.Func {
		return errors.New("only functions can be bound")
	}
	if n := v.Type().NumOut(); n > 2 {
		return errors.New("function may only return a value or a value+error")
	}
	w.m.Lock()
	w.bindings[name] = f
	w.m.Unlock()

	w.Init("(function() { var name = " + jsString(name) + ";" + `
		var RPC = window._rpc = (window._rpc || {nextSeq: 1});
		window[name] = function() {
		  var seq = RPC.nextSeq++;
		  var promise = new Promise(function(resolve, reject) {
			RPC[seq] = {
			  resolve: resolve,
			  reject: reject,
			};
		  });
		  window.external.invoke(JSON.stringify({
			id: seq,
			method: name,
			params: Array.prototype.slice.call(arguments),
		  }));
		  return promise;
		}
	})()`)

	return nil
}

func (w *webview) Hide() {
	w32.User32ShowWindow.Call(w.hwnd, w32.SWHide)
}

func (w *webview) Show() {
	w32.User32ShowWindow.Call(w.hwnd, w32.SWShow)
	w32.User32SwitchToThisWindow.Call(w.hwnd, uintptr(1))
}

func (w *webview) onNavigationCompleted(h func(args *navigationCompletedArg)) {
	w.browser.(*edge.Chromium).OnNavigationCompleted(func(sender *edge.ICoreWebView2, args *edge.ICoreWebView2NavigationCompletedEventArgs) {
		h(&navigationCompletedArg{Success: args.IsSuccess()})
	})
}

func (w *webview) SetFallbackPage(html string) error {
	chromium := w.browser.(*edge.Chromium)
	chromium.PutIsBuiltInErrorPageEnabled(false)
	w.onNavigationCompleted(func(args *navigationCompletedArg) {
		if !args.Success {
			w.SetHtml(html)
		}
	})
	return nil
}
