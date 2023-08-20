package webview2

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/eyasliu/desktop/go-webview2/pkg/edge"
)

type iStreamReleaseCloser struct {
	stream *edge.IStream
	closed bool
}

func (i *iStreamReleaseCloser) Read(p []byte) (int, error) {
	if i.closed {
		return 0, io.ErrClosedPipe
	}
	return i.stream.Read(p)
}

func (i *iStreamReleaseCloser) Close() error {
	if i.closed {
		return nil
	}
	i.closed = true
	return i.stream.Release()
}

type edgeOnNavigateHandler = func(request *edge.ICoreWebView2WebResourceRequest, args *edge.ICoreWebView2WebResourceRequestedEventArgs)

func coreWebview2RequestToHttpRequest(coreReq *edge.ICoreWebView2WebResourceRequest) (*http.Request, error) {

	header := http.Header{}
	headers, err := coreReq.GetHeaders()
	if err != nil {
		return nil, fmt.Errorf("GetHeaders Error: %s", err)
	}
	defer headers.Release()

	headersIt, err := headers.GetIterator()
	if err != nil {
		return nil, fmt.Errorf("GetIterator Error: %s", err)
	}
	defer headersIt.Release()

	for {
		has, err := headersIt.HasCurrentHeader()
		if err != nil {
			return nil, fmt.Errorf("HasCurrentHeader Error: %s", err)
		}
		if !has {
			break
		}

		name, value, err := headersIt.GetCurrentHeader()
		if err != nil {
			return nil, fmt.Errorf("GetCurrentHeader Error: %s", err)
		}

		header.Set(name, value)
		if _, err := headersIt.MoveNext(); err != nil {
			return nil, fmt.Errorf("MoveNext Error: %s", err)
		}
	}

	// WebView2 has problems when a request returns a 304 status code and the WebView2 is going to hang for other
	// requests including IPC calls.
	// So prevent 304 status codes by removing the headers that are used in combinationwith caching.
	header.Del("If-Modified-Since")
	header.Del("If-None-Match")

	method, err := coreReq.GetMethod()
	if err != nil {
		return nil, fmt.Errorf("GetMethod Error: %s", err)
	}

	uri, err := coreReq.GetUri()
	if err != nil {
		return nil, fmt.Errorf("GetUri Error: %s", err)
	}

	var body io.ReadCloser
	if content, err := coreReq.GetContent(); err != nil {
		return nil, fmt.Errorf("GetContent Error: %s", err)
	} else if content != nil {
		body = &iStreamReleaseCloser{stream: content}
	}

	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		if body != nil {
			body.Close()
		}
		return nil, err
	}
	req.Header = header
	return req, nil
}

func (w *Window) processRequest(req *edge.ICoreWebView2WebResourceRequest, args *edge.ICoreWebView2WebResourceRequestedEventArgs) {
	//Get the request
	uri, _ := req.GetUri()
	reqUri, _ := url.Parse(uri)

	var isMatch = false
	var handler http.HandlerFunc

	for rule, h := range w.serverRoute {
		isMatch = testMatchPath(reqUri, rule)
		if isMatch {
			handler = h
			break
		}
	}
	if !isMatch {
		return
	}

	rw := httptest.NewRecorder()
	request, err := coreWebview2RequestToHttpRequest(req)

	if err != nil {
		w.webview.logger.Info("parse http request fail", err)
		return
	}

	handler(rw, request)

	headers := []string{}
	for k, v := range rw.Header() {
		headers = append(headers, fmt.Sprintf("%s: %s", k, strings.Join(v, ",")))
	}

	code := rw.Code
	if code == http.StatusNotModified {
		// WebView2 has problems when a request returns a 304 status code and the WebView2 is going to hang for other
		// requests including IPC calls.
		w.webview.logger.Info("%s: AssetServer returned 304 - StatusNotModified which are going to hang WebView2, changed code to 505 - StatusInternalServerError", uri)
		code = http.StatusInternalServerError
	}

	env := w.webview.browser.Environment()
	response, err := env.CreateWebResourceResponse(rw.Body.Bytes(), code, http.StatusText(code), strings.Join(headers, "\n"))
	if err != nil {
		w.webview.logger.Info("CreateWebResourceResponse Error: %s", err)
		return
	}
	defer response.Release()

	// Send response back
	err = args.PutResponse(response)
	if err != nil {
		w.webview.logger.Info("PutResponse Error: %s", err)
		return
	}
	fmt.Println("end req", uri)
}

// ServerHandleFunc 使用函数绑定路由规则
func (w *Window) ServerHandleFunc(pattern string, handler http.HandlerFunc) {
	// if len(w.serverRoute) == 0 {
	w.webview.browser.AddWebResourceRequestedFilter(pattern, edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL)
	// }
	w.serverRoute[pattern] = handler
}

// ServerHandle 使用 http 服务器绑定路由
func (w *Window) ServerHandle(pattern string, handler http.Handler) {
	w.ServerHandleFunc(pattern, handler.ServeHTTP)
}
