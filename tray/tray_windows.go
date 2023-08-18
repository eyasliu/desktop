//go:build windows
// +build windows

package tray

import (
	"runtime"

	"github.com/eyasliu/desktop/tray/systray"
)

// 托盘菜单项
type TrayItem struct {
	ins    *systray.MenuItem
	parent *TrayItem
	// 菜单标题，显示在菜单列表
	Title string
	// 菜单提示文字，好像没有显示
	Tooltip string
	// 是否有复选框
	Checkbox bool
	// 复选框是否已选中
	Checked bool
	// 是否被禁用，禁用后不可点击
	Disable bool
	// 点击菜单触发的回调函数
	OnClick func()
	// 子菜单项
	Items []*TrayItem
}

func (ti *TrayItem) register() {
	if ti.parent == nil {
		if ti.Checkbox {
			ti.ins = systray.AddMenuItemCheckbox(ti.Title, ti.Tooltip, ti.Checked)
		} else {
			ti.ins = systray.AddMenuItem(ti.Title, ti.Tooltip)
		}
	} else {
		if ti.Checkbox {
			ti.ins = ti.parent.ins.AddSubMenuItemCheckbox(ti.Title, ti.Tooltip, ti.Checked)
		} else {
			ti.ins = ti.parent.ins.AddSubMenuItem(ti.Title, ti.Tooltip)
		}
	}

	if ti.Disable {
		ti.ins.Disable()
	}
	if ti.OnClick != nil {
		go func(menu *TrayItem) {
			for {
				<-ti.ins.ClickedCh
				ti.OnClick()
			}
		}(ti)
	}
	ti.setSubItems()
}

func (ti *TrayItem) setSubItems() {
	if len(ti.Items) == 0 {
		return
	}
	for _, sub := range ti.Items {
		sub.parent = ti
		sub.register()
	}
}

// Update 更新托盘菜单状态，调用前自行修改 TrayItem 实例的属性值
func (ti *TrayItem) Update() {
	ti.ins.SetInfo(ti.Title, ti.Tooltip, ti.Checked, ti.Checkbox, ti.Disable)
}

// Tray 系统托盘配置
type Tray struct {
	// 托盘图标路径，请注意要使用 ico 格式的图片，会继承自 desktop.Option，如果设置了会覆盖
	IconPath string
	// 托盘图标内容，请注意要使用 ico 格式的图片，会继承自 desktop.Option，如果设置了会覆盖
	IconBytes []byte
	// 托盘标题，也不知道在哪显示
	Title string
	// 托盘提示文字，鼠标移到托盘图标时显示
	Tooltip string
	// 右键托盘图标显示的菜单项
	Items []*TrayItem
	// 单机托盘图标时触发的回调函数
	OnClick func()
}

// SetIconBytes 设置图标内容，请注意要使用 ico 格式的图片
func (t *Tray) SetIconBytes(img []byte) {
	t.IconBytes = img
	systray.SetIcon(t.IconBytes)
}

// SetIconPath 设置图标路径，请注意要使用 ico 格式的图片
func (t *Tray) SetIconPath(path string) {
	t.IconPath = path
	systray.SetIconPath(t.IconPath)
}

// SetTooltip 更新托盘提示文字
func (t *Tray) SetTooltip(text string) {
	t.Tooltip = text
	systray.SetTooltip(t.Tooltip)
}

// SetTitle 更新托盘标题
func (t *Tray) SetTitle(text string) {
	t.Title = text
	systray.SetTitle(t.Title)
}

// Run 开始初始化托盘功能，该方法是阻塞的
func Run(t *Tray) {
	runtime.LockOSThread()
	systray.Run(t.onReady, nil)
	runtime.UnlockOSThread()
}

func (t *Tray) onReady() {
	if t.IconPath != "" {
		systray.SetIconPath(t.IconPath)
	} else if len(t.IconBytes) > 1 {
		systray.SetIcon(t.IconBytes)
	}
	if t.Title != "" {
		systray.SetTitle(t.Title)
	}
	if t.Tooltip != "" {
		systray.SetTooltip(t.Tooltip)
	}
	if t.OnClick != nil {
		systray.SetOnClick(t.OnClick)
	}
	if len(t.Items) > 0 {
		for _, menu := range t.Items {
			menu.register()
		}
	}
}

// Quit 退出托盘功能
func Quit() {
	systray.Quit()
}
