package gogaecms

import (
	"appengine"
	"appengine/blobstore"
	"appengine/datastore"
	"appengine/memcache"
	"appengine/urlfetch"
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"path/filepath"
	"html/template"
	"io"
	"os"
	"encoding/csv"
	"encoding/gob"
)
type Content struct {
	Data    []byte
	ETag    string
	BlobKey string
}



func init() {
	http.HandleFunc("/update", updateHandler)
	http.HandleFunc("/", handler)
}

func isDevelopment() bool {
	return appengine.IsDevAppServer()
}
type DataRecord []string
type DataEntities map[string][]DataRecord

func loadCSV(file io.Reader) ([]DataRecord,error) {
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
func loadData() (DataEntities, error) {
	files, err := filepath.Glob("site/data/*")
	if err!=nil {
		return nil, err
	}
	result := make(DataEntities)
	for n := range(files) {
		file, err := os.Open(files[n])
		if err != nil {
			return nil, err
		}
		defer file.Close()

		records, err := loadCSV(file)
		if err!=nil {
			return nil, err
		}
		result["items"] = records
	}
	return result, nil
}

func serveDevelopment(w http.ResponseWriter, r *http.Request, fileName string) {
	fileName = fileName[1:]
	if !strings.HasSuffix(fileName,".html") {
		http.ServeFile(w,r,filepath.Join("site",fileName));
	} else {
		pattern := filepath.Join("site", "*.html")
		tmpl := template.Must(template.ParseGlob(pattern))
		w.Header().Add("Content-Type", "text/html")
		data, err := loadData()
		if err!= nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		err = tmpl.ExecuteTemplate(w,fileName,data)
		if err!=nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

var templates *template.Template

func loadTemplates(c appengine.Context) (*template.Template, error) {
	q := datastore.NewQuery("Content").
		Filter("BlobKey =", "")
	templates := template.New("root");

	var contents []Content
	if keys, err := q.GetAll(c, &contents); err != nil {
		return nil,err
	} else {
		for i := range(contents) {
			var k *datastore.Key = keys[i]
			name := k.StringID()[1:]
			_,err = templates.New(name).Parse(string(contents[i].Data[:]))
			if err != nil {
				return nil,err
			}
		}
		return templates,nil
	}

}
var datas DataEntities
func loadDatas(c appengine.Context) (DataEntities, error) {
	//result := DataEntities{"test":{{"a","zxc"}}}
	var result DataEntities
	var key = datastore.NewKey(c, "Data","default",0, nil);
	var dataStore DataStoreRecord
	if err := datastore.Get(c, key, &dataStore); err != nil {
		return nil, err
	}
	buff := bytes.NewBuffer(dataStore.Bytes)
	dec := gob.NewDecoder(buff)
	dec.Decode(&result)
	return result, nil
}
func transformTemplate(c appengine.Context,fileName string, content *Content) error {
	var err error;
	if templates == nil {
		templates, err = loadTemplates(c)
		if err != nil {
			return err
		}
	}
	if datas == nil {
		datas, err = loadDatas(c)
		if err != nil {
			return err
		}
	}
	buf := new(bytes.Buffer)
	templates.ExecuteTemplate(buf, fileName[1:], datas)
	content.Data = buf.Bytes()
    return nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	fileName := r.URL.Path
	if fileName == "/" {
		fileName = "/index.html"
	}
	/*if isDevelopment() {
		serveDevelopment(w, r, fileName)
		return
	}*/
	var content Content
	c := appengine.NewContext(r)

	item, err := memcache.Gob.Get(c, fileName, &content)
	if err != nil && err != memcache.ErrCacheMiss {
		c.Errorf("error getting item: %v", err)
	}
	if item == nil {
		key := datastore.NewKey(c, "Content", fileName, 0, nil)
		if err := datastore.Get(c, key, &content); err != nil {
			if err == datastore.ErrNoSuchEntity {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if strings.HasSuffix(fileName,".html") {
			transformTemplate(c, fileName, &content)
		}
		item := &memcache.Item{
			Key:    fileName,
			Object: content,
		}
		err = memcache.Gob.Add(c, item)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if nonMatch := r.Header.Get("If-None-Match"); nonMatch == content.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	mimeType := mime.TypeByExtension(fileName[strings.LastIndex(fileName, "."):])
	w.Header().Add("ETag", content.ETag)
	if content.BlobKey == "" {
		w.Header().Add("Content-Type", mimeType)
		w.Write(content.Data)
	} else {
		blobstore.Send(w, appengine.BlobKey(content.BlobKey))
	}
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	// url https://github.com/olostan/test-website/zipball/master
	c := appengine.NewContext(r)
	client := urlfetch.Client(c)
	resp, err := client.Get("https://github.com/olostan/test-website/zipball/master")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	dataReader := bytes.NewReader(data)

	reader, err := zip.NewReader(dataReader, resp.ContentLength)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	//defer reader.Close()

	err = removeAllContent(c, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	newData := DataEntities{}

	for _, f := range reader.File {

		fileName := strings.SplitN(f.Name, "/", 2)[1]

		fmt.Fprintf(w, "Importing '%s':", fileName)
		if len(fileName) == 0 || f.FileInfo().IsDir() {
			fmt.Fprintf(w, "Skip.\n")
			continue
		}
		rc, err := f.Open()
		defer rc.Close()

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			continue
		}
		var content Content
		if strings.HasPrefix(fileName,"data") {
			fmt.Fprintf(w, "used as data.\n")
			entities, err := loadCSV(rc)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				continue
			}
			var dataName = fileName[5:len(fileName)-4]
			newData[dataName] = entities
			continue;
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(rc)

		if f.FileInfo().Size() > 1024*10 {
			content = Content{
				Data: nil,
			}
			blobWriter, err := blobstore.Create(c, mime.TypeByExtension(fileName[strings.LastIndex(fileName, "."):]))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				continue
			}
			if _, err = blobWriter.Write(buf.Bytes()); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				continue
			}
			if err = blobWriter.Close(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				continue
			}
			blobKey, err := blobWriter.Key()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				continue
			}
			content.BlobKey = string(blobKey)
		} else {
			content = Content{
				Data: buf.Bytes(),
			}
		}
		content.ETag = strconv.FormatUint(uint64(f.CRC32), 16)
		//fmt.Fprintf(w,"[%s]\n", content)

		key := datastore.NewKey(c, "Content", "/"+fileName, 0, nil)
		_, err = datastore.Put(c, key, &content)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		fmt.Fprintf(w, "Ok.\n")
	}

	buff := new(bytes.Buffer)
	enc := gob.NewEncoder(buff)

	err = enc.Encode(&newData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dataStore := DataStoreRecord{buff.Bytes()}
	_, err = datastore.Put(c, datastore.NewKey(c, "Data","default",0, nil),&dataStore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
type DataStoreRecord struct {
  Bytes []byte
}
func removeAllContent(c appengine.Context, w http.ResponseWriter) error {
	memcache.Flush(c)
	q := datastore.NewQuery("Content").
		Filter("BlobKey >", "")
	var contents []Content
	if _, err := q.GetAll(c, &contents); err != nil {
		return err
	}
	blobKeys := make([]appengine.BlobKey, len(contents))
	for c := range contents {
		fmt.Fprintf(w, "Deleted file %s\n", contents[c].BlobKey)
		blobKeys[c] = appengine.BlobKey(contents[c].BlobKey)
	}
	if err := blobstore.DeleteMulti(c, blobKeys); err != nil {
		return err
	}

	keys, err := datastore.NewQuery("Content").KeysOnly().GetAll(c, nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Deleted %d entities\n", len(keys))
	err = datastore.DeleteMulti(c, keys)
	return err
}

