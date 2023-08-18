//go:build !windows
// +build !windows

// only windows support

package tray

type TrayItem struct {
	Title    string
	Tooltip  string
	Checkbox bool
	Checked  bool
	Disable  bool
	OnClick  func()
	Items    []TrayItem
}

type Tray struct {
	Icon    []byte
	Title   string
	Tooltip string
	OnClick func()
	Items   []*TrayItem
}

func Quit() {}
