package edge

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type _ICoreWebView2NavigationCompletedEventArgsVtbl struct {
	_IUnknownVtbl
	GetIsSuccess      ComProc
	GetWebErrorStatus ComProc
	GetNavigationId   ComProc
}

type ICoreWebView2NavigationCompletedEventArgs struct {
	vtbl *_ICoreWebView2NavigationCompletedEventArgsVtbl
}

func (i *ICoreWebView2NavigationCompletedEventArgs) AddRef() uintptr {
	r, _, _ := i.vtbl.AddRef.Call()
	return r
}

func (i *ICoreWebView2NavigationCompletedEventArgs) IsSuccess() bool {
	var err error
	var enabled bool
	_, _, err = i.vtbl.GetIsSuccess.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&enabled)),
	)
	if err != windows.ERROR_SUCCESS {
		return false
	}
	return enabled
}

type COREWEBVIEW2_WEB_ERROR_STATUS uint32

func (i *ICoreWebView2NavigationCompletedEventArgs) GetWebErrorStatus() COREWEBVIEW2_WEB_ERROR_STATUS {
	var err error
	var enabled COREWEBVIEW2_WEB_ERROR_STATUS
	_, _, err = i.vtbl.GetWebErrorStatus.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&enabled)),
	)
	if err != windows.ERROR_SUCCESS {
		return 0
	}
	return enabled
}

func (i *ICoreWebView2NavigationCompletedEventArgs) GetNavigationId() int32 {
	var err error
	var enabled int32
	_, _, err = i.vtbl.GetNavigationId.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&enabled)),
	)
	if err != windows.ERROR_SUCCESS {
		return -1
	}
	return enabled
}
