package webview2

import (
	"net"
	"net/url"
	"strings"
	"sync"
)

type matchPath struct {
	isRevese       bool // 是否反匹配
	wildcardFixStr string
}

var matchPathCache = map[string]*matchPath{}
var matchPathSyncmux sync.RWMutex

// 访问的URL是否能匹配上配置的URL
// example.com:8080 会转换为 https://example:8080* 做通配符匹配
// !/api 会转换为 */api* 做通配符反匹配
func testMatchPath(requestUrl *url.URL, rule string) bool {
	// matchUrl := requestUrl.String()
	// 匹配时只匹配域名和路径，不匹配查询参数

	paths := strings.Split(rule, ",")
	passCount := 0
	sphost, spport, _ := net.SplitHostPort(requestUrl.Host)
	hostHasPort := spport != ""
	hostHasDefPort := (spport == "80" || spport == "443")
	for _, path := range paths {
		host := requestUrl.Host
		confPathHasPort := strings.Contains(path, ":")
		confUrl, err := url.Parse(path)
		if err == nil {
			confPathHasPort = strings.Contains(confUrl.Host, ":")
		}

		// 如果配置带了端口号，则需要补全默认端口号去做匹配，如果配置没有带端口号，则把默认的端口号去掉，简化路径配置规则

		if !confPathHasPort && hostHasDefPort {
			host = sphost
		} else if confPathHasPort && !hostHasPort {
			if requestUrl.Scheme == "https" {
				host += ":443"
			} else {
				host += ":80"
			}
		}
		matchUrl := requestUrl.Scheme + "://" + host + requestUrl.Path
		if path == "" {
			passCount++
			continue
		}
		var mp *matchPath
		var ok bool
		matchPathSyncmux.RLock()
		mp, ok = matchPathCache[path]
		matchPathSyncmux.RUnlock()
		if !ok {
			isrev := path[0:1] == "!"
			fixstr := path

			if isrev {
				fixstr = strings.Replace(path, "!", "", 1)
			}
			if !strings.Contains(fixstr, "://") {
				sp := strings.Split(fixstr, "/")
				isDomainStart := false
				if len(sp) > 0 {
					isDomainStart = sp[0] != "" && (strings.Contains(sp[0], ".") || sp[0] == "localhost")
				}
				if strings.Contains(fixstr, ":") || isDomainStart {
					fixstr = "http*://" + fixstr
				} else {
					fixstr = "*" + fixstr
				}
			}
			if strings.LastIndex(fixstr, "*") != len(fixstr)-1 {
				fixstr += "*"
			}
			mp = &matchPath{
				isRevese:       isrev,
				wildcardFixStr: fixstr,
			}
			matchPathSyncmux.Lock()
			matchPathCache[path] = mp
			matchPathSyncmux.Unlock()
		}
		isPass := wildcardMatch(matchUrl, mp.wildcardFixStr)
		if (mp.isRevese && !isPass) || (!mp.isRevese && isPass) {
			passCount++
		}
	}
	if passCount == len(paths) {
		return true
	}
	return false
}

// WildcardMatch 通配符匹配
// s 待匹配的字符串
// p 匹配规则
func wildcardMatch(s string, p string) bool {
	sLen, pLen := len(s), len(p)
	res := make([][]bool, 2)
	for i := 0; i < 2; i++ {
		res[i] = make([]bool, pLen+1)
	}
	res[0][0] = true
	for i := 0; i <= sLen; i++ {
		for j := 1; j <= pLen; j++ {
			match := (i > 0 && s[i-1] == p[j-1]) || p[j-1] == '?' || p[j-1] == '*'
			res[i&1][j] = i > 0 && res[(i-1)&1][j-1] && match
			if p[j-1] == '*' {
				res[i&1][j] = res[i&1][j-1] || (i > 0 && res[(i-1)&1][j])
			}
		}
		if i > 1 {
			res[0][0] = false
		}
	}
	return res[sLen&1][pLen]
}
