package gogaecms

import (
	"bytes"
	"net/http"
	"strings"
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
			templates, err := contentHandler.LoadTemplates(&request)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			buf := new(bytes.Buffer)
			err = templates.ExecuteTemplate(buf, name, datas)
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
