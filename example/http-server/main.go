package main

import (
	"fmt"
	"net/http"

	"github.com/eyasliu/desktop"
)

var navHtml = `<ul>
<li><a href='http://default.local'>http://default.local</a></li>
<li><a href='http://my.local'>http://my.local</a></li>
<li><a href='http://notdefined.local/demo'>http://notdefined.local/demo</a></li>
<li><a href='http://baidu.com'>baidu.com</a></li>
<li><a href='http://localhost:3345'>3345</a></li>
<li><a href='http://localhost:3346'>3346</a></li>
</ul>`

func main() {
	opt := &desktop.Options{
		Debug:    true,
		StartURL: "http://localhost:3345",
	}
	app := desktop.New(opt)

	// 访问路径 /demo 时命中，无视域名
	// app.ServerHandleFunc("*/demo", func(w http.ResponseWriter, r *http.Request) {
	// 	w.Header().Set("content-type", "text/html;charset=utf-8")
	// 	w.WriteHeader(200)
	// 	w.Write([]byte("这是 /demo 路径页面，无论那个页面访问都能命中" + navHtml))
	// })

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("default mux server")
		w.Header().Set("content-type", "text/html;charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte(r.Host + `, 这是标准库 net/http 的默认 http 实例` + navHtml))
	})
	// 访问到 http://default.local 域名时使用标准库默认的 http 服务器实例
	app.ServerHandle("*default.local/*", http.DefaultServeMux)

	myserver := http.NewServeMux()
	myserver.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/html;charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte(`hello, 这是我自己定义的 http 实例` + navHtml))
	})
	// 访问到 my.local 域名时使用自定义的http实例
	// app.ServerHandle("*my.local/*", myserver)

	server := http.Server{
		Addr:    "0.0.0.0:3346",
		Handler: myserver,
	}
	go server.ListenAndServe()
	go http.ListenAndServe("0.0.0.0:3345", nil)

	app.Run()
}
