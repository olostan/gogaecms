package gogaecms

import "net/http"
import "html/template"

type Content struct {
	Data    []byte
	ETag    string
	BlobKey string
}

type DataStoreRecord struct {
	Bytes []byte
}

type DataRecord []string
type DataEntities map[string][]DataRecord
type Context interface{}

type Request struct {
	w        http.ResponseWriter
	r        *http.Request
	fileName string
	context  Context
}

type ContentHandler interface {
	NewContext(req *Request) Context
	ServeStatic(req *Request)
	GetFromCache(req *Request) *Content
	Get(req *Request) *Content
	PutToCache(req *Request, content *Content)
	SendContent(req *Request, content *Content)

	LoadDatas(req *Request) (DataEntities, error)
	LoadTemplates(req *Request,root *template.Template) (*template.Template, error)
}
