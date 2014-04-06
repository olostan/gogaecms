package gogaecms

import (
	"appengine"
	"appengine/blobstore"
	"appengine/datastore"
	"appengine/memcache"
	"bytes"
	"encoding/gob"
	"html/template"
	"net/http"
	"mime"
	"strings"
)

type GAEHandler struct {
}

func (h *GAEHandler) NewContext(req *Request) Context {
	var c = appengine.NewContext(req.r)
	return c
}

func (h *GAEHandler) ServeStatic(_ *Request) {
}
func (h *GAEHandler) GetFromCache(req *Request) *Content {
	var content Content
	var c = req.context.(appengine.Context)
	_, err := memcache.Gob.Get(c, req.fileName, &content)
	if err != nil && err != memcache.ErrCacheMiss {
		c.Errorf("error getting item: %v", err)
	}
	if err == memcache.ErrCacheMiss {
		return nil
	}
	return &content
}
func (h *GAEHandler) Get(req *Request) *Content {
	var c = req.context.(appengine.Context)
	key := datastore.NewKey(c, "Content", req.fileName, 0, nil)
	var content Content
	if err := datastore.Get(c, key, &content); err != nil {
		if err == datastore.ErrNoSuchEntity {
			http.NotFound(req.w, req.r)
			return nil
		}
		http.Error(req.w, err.Error(), http.StatusInternalServerError)
		return nil
	}
	/*if strings.HasSuffix(fileName,".html") {
		transformTemplate(c, fileName, &content)
	}*/
	return &content
}

func (h *GAEHandler) PutToCache(req *Request, content *Content) {
	item := &memcache.Item{
		Key:    req.fileName,
		Object: content,
	}
	err := memcache.Gob.Add(req.context.(appengine.Context), item)
	if err != nil {
		http.Error(req.w, err.Error(), http.StatusInternalServerError)
		return
	}
}
func (h *GAEHandler) SendContent(req *Request, content *Content) {
	mimeType := mime.TypeByExtension(req.fileName[strings.LastIndex(req.fileName, "."):])
	req.w.Header().Add("Cache-Control", "public")

	if content.BlobKey == "" {
		req.w.Header().Add("ETag", content.ETag)
		req.w.Header().Add("Content-Type", mimeType)

		req.w.Write(content.Data)
	} else {
		blobstore.Send(req.w, appengine.BlobKey(content.BlobKey))
	}
}

func (h *GAEHandler) LoadDatas(req *Request) (DataEntities, error) {
	//result := DataEntities{"test":{{"a","zxc"}}}
	var result DataEntities
	var key = datastore.NewKey(req.context.(appengine.Context), "Data", "default", 0, nil)
	var dataStore DataStoreRecord
	if err := datastore.Get(req.context.(appengine.Context), key, &dataStore); err != nil {
		return nil, err
	}
	buff := bytes.NewBuffer(dataStore.Bytes)
	dec := gob.NewDecoder(buff)
	dec.Decode(&result)
	return result, nil
}
func (h *GAEHandler) LoadTemplates(req *Request) (*template.Template, error) {
	q := datastore.NewQuery("Content").
		Filter("BlobKey =", "")
	templates := template.New("root")

	var contents []Content
	if keys, err := q.GetAll(req.context.(appengine.Context), &contents); err != nil {
		return nil, err
	} else {
		for i := range contents {
			var k *datastore.Key = keys[i]
			name := k.StringID()[1:]
			_, err = templates.New(name).Parse(string(contents[i].Data[:]))
			if err != nil {
				return nil, err
			}
		}
		return templates, nil
	}
}
