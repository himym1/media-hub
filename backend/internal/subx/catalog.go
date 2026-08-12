package subx

type Operation struct {
	ID          string      `json:"id"`
	Group       string      `json:"group"`
	Label       string      `json:"label"`
	Method      string      `json:"-"`
	Path        string      `json:"-"`
	Query       []Parameter `json:"query"`
	PathParams  []Parameter `json:"pathParams"`
	Command     bool        `json:"command"`
	Body        bool        `json:"body"`
	Destructive bool        `json:"destructive"`
}

type Parameter struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
}

func stringParam(name string, required bool) Parameter {
	return Parameter{Name: name, Kind: "string", Required: required}
}

func read(id, group, label, path string, query ...Parameter) Operation {
	return Operation{ID: id, Group: group, Label: label, Method: "GET", Path: path, Query: append([]Parameter{}, query...), PathParams: []Parameter{}}
}

func command(id, group, label, path string) Operation {
	return Operation{ID: id, Group: group, Label: label, Method: "POST", Path: path, Command: true, Body: true, Query: []Parameter{}, PathParams: []Parameter{}}
}

var operationList = []Operation{
	read("health", "system", "SubX 健康状态", "/api/health"),
	read("contents.backup.export", "content", "导出内容备份", "/api/contents/backup/export"),
	read("sources.search", "sources", "跨来源搜索", "/api/sources/search", stringParam("keyword", false), stringParam("source_id", false)),
	command("dian.save", "sources", "转存点点资源", "/api/dian/save"),
	command("framehdr.save", "sources", "转存 FrameHDR 资源", "/api/framehdr/save"),
	command("gimy.save", "sources", "转存 Gimy 资源", "/api/gimy/save"),
	command("guanying.save", "sources", "转存观影资源", "/api/guanying/save"),
	command("hdhive.save", "sources", "转存 HDHive 资源", "/api/hdhive/save"),
	command("juying.save", "sources", "转存聚影资源", "/api/juying/save"),
	command("mikan.save", "sources", "转存蜜柑资源", "/api/mikan/save"),
	command("sidhub.save", "sources", "转存 Sidhub 资源", "/api/sidhub/save"),
}

func LookupOperation(id string) (Operation, bool) {
	for _, operation := range operationList {
		if operation.ID == id {
			return operation, true
		}
	}
	return Operation{}, false
}
