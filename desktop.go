//go:build windows
// +build windows

package desktop

import (
	"github.com/eyasliu/desktop/tray"

	"github.com/eyasliu/desktop/go-webview2"
)

// New 新建一个 webview 窗口
func New(opt *Options) WebView {
	// 托盘图标
	iconpath := opt.GetIcon()
	if opt.Tray != nil {
		if opt.Tray.IconPath == "" {
			opt.Tray.IconPath = opt.IconPath
		}
		if len(opt.Tray.IconBytes) == 0 {
			opt.Tray.IconBytes = opt.IconBytes
		}
		if opt.Tray.IconPath == "" && len(opt.Tray.IconBytes) == 0 {
			opt.Tray.IconPath = iconpath
		}
	}

	wvOpts := webview2.WebViewOptions{
		Debug:             opt.Debug,
		StartURL:          opt.StartURL,
		FallbackPage:      opt.FallbackPage,
		DataPath:          opt.DataPath,
		AutoFocus:         opt.AutoFocus,
		HideWindowOnClose: opt.HideWindowOnClose,
		Logger:            opt.Logger,
		WindowOptions: webview2.WindowOptions{
			Icon:      iconpath,
			Frameless: opt.Frameless,
			Title:     opt.Title,
			Center:    opt.Center,
			Width:     uint(opt.Width),
			Height:    uint(opt.Height),
		},
	}
	if wvOpts.Logger == nil {
		wvOpts.Logger = &defaultLogger{}
	}

	if IsSupportTray() && opt.Tray != nil {
		go tray.Run(opt.Tray)
	}
	w := webview2.NewWin(wvOpts, opt.Tray)
	return w
}
