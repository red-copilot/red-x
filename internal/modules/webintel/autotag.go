package webintel

import (
	"strings"

	"github.com/red-copilot/red-x/internal/modules/discovery"
)

func AutoTag(site *discovery.SiteResult) {
	if site == nil {
		return
	}
	title := strings.ToLower(site.Title)
	invalidTitles := []string{
		"welcome to nginx", "iis7", "apache tomcat", "welcome to centos",
		"apache http server test page", "test page for the nginx http",
		"500 internal server error", "error 404--not found", "welcome to openresty",
		"没有找到站点", "404 not found", "页面不存在", "访问拦截", "403 forbidden", "page not found",
	}
	for _, t := range invalidTitles {
		if strings.Contains(title, t) {
			site.Tags = []string{"无效"}
			return
		}
	}
	if site.BodyLength <= 300 && site.Title == "" && site.Status >= 400 {
		site.Tags = []string{"无效"}
		return
	}
	if site.Status == 301 || site.Status == 302 || site.Status == 303 {
		loc := site.Headers["location"]
		if loc == "" || strings.Contains(loc, site.Hostname) {
			site.Tags = []string{"无效"}
			return
		}
	}
	site.Tags = []string{"入口"}
}
