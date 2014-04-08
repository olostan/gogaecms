package gogaecms

import (
	"bytes"
	"net/http"
	"strings"
	"html/template"
)

var contentHandler ContentHandler

func init() {

	contentHandler = NewLocalHandler("site")
	//	contentHandler = new(GAEHandler);
	http.HandleFunc("/update", updateHandler)
	http.HandleFunc("/", handler)

}

func handler(w http.ResponseWriter, r *http.Request) {
	fileName := r.URL.Path
	if fileName == "/" {
		fileName = "/index.html"
	}
	request := Request{w: w, r: r, fileName: fileName}

	request.context = contentHandler.NewContext(&request)

	/*if !strings.HasSuffix(fileName, ".html") {
		contentHandler.ServeStatic(&request);
		return;
	}*/

	var content *Content

	content = contentHandler.GetFromCache(&request)

	if content == nil {
		content = contentHandler.Get(&request)

		if content == nil {
			w.Write([]byte("Not content found!"))
			return
		}
		if strings.HasSuffix(fileName, ".html") {
			name := fileName[1:]
			println("Transfoming template", name)
			datas, err := contentHandler.LoadDatas(&request)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			root := template.New("root");
			funcMap := template.FuncMap{
				"data": func (storeName,keyName string) string {
					store := datas[storeName];
					for _,row := range store {
						if row[0] == keyName {
							return row[1];
						}
					}
					return ""
				},
			}
			root.Funcs(funcMap);
			templates, err := contentHandler.LoadTemplates(&request,root)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
//			datas["global"] = []DataRecord{[]string{"file",fileName[1:]}};
			current := templates.Lookup(fileName[1:]);
			templates.AddParseTree("body",current.Tree);
			buf := new(bytes.Buffer)
			//err = templates.ExecuteTemplate(buf, name, datas)
			err = templates.ExecuteTemplate(buf, "layout.html", datas)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			println("Transfomed template", name, len(buf.Bytes()))
			content.Data = buf.Bytes()
		}
		contentHandler.PutToCache(&request, content)
	}

	if nonMatch := r.Header.Get("If-None-Match"); nonMatch != "" && nonMatch == content.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	contentHandler.SendContent(&request, content)
}
