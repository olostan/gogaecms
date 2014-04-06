package gogaecms

import (
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"io/ioutil"
)

type LocalHandler struct {
	folder string
}

func NewLocalHandler(folder string) *LocalHandler {
	return &LocalHandler{folder: folder}
}

func (h *LocalHandler) NewContext(_ *Request) Context {
	return nil
}

func (h *LocalHandler) ServeStatic(req *Request) {
	http.ServeFile(req.w, req.r, filepath.Join(h.folder, req.fileName))
}
func (h *LocalHandler) GetFromCache(_ *Request) *Content {
	return nil
}
func (h *LocalHandler) Get(req *Request) *Content {
	name := filepath.Join(h.folder, req.fileName)
	bytes, err := ioutil.ReadFile(name)
	if err != nil {
		return nil
	}
	md5Sum := md5.Sum(bytes)
	etag := hex.EncodeToString(md5Sum[:])
	content := Content{Data: bytes, ETag: etag}
	return &content
}
func (h *LocalHandler) PutToCache(_ *Request, _ *Content) {

}

func (h *LocalHandler) SendContent(req *Request, content *Content) {
	req.w.Header().Add("Content-Type", "text/html")
	req.w.Header().Add("ETag", content.ETag)
	req.w.Write(content.Data)

}

func (h *LocalHandler) LoadDatas(_ *Request) (DataEntities, error) {
	files, err := filepath.Glob("site/data/*")
	if err != nil {
		return nil, err
	}
	result := make(DataEntities)
	for n := range files {
		file, err := os.Open(files[n])
		if err != nil {
			return nil, err
		}
		defer file.Close()

		records, err := loadCSV(file)
		if err != nil {
			return nil, err
		}
		result["items"] = records
	}
	return result, nil
}

func loadCSV(file io.Reader) ([]DataRecord, error) {
	reader := csv.NewReader(file)
	var result []DataRecord
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}

func (h *LocalHandler) LoadTemplates(_ *Request) (*template.Template, error) {
	pattern := filepath.Join(h.folder, "*.html")
	template := template.Must(template.ParseGlob(pattern))
	return template, nil
}
