package gogaecms

import (
	"appengine"
	"appengine/blobstore"
	"appengine/datastore"
	"appengine/memcache"
	"appengine/urlfetch"
	"archive/zip"
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

func loadDatas(reader *zip.Reader, w *http.ResponseWriter) *DataEntities {
	newData := DataEntities{}
	fmt.Fprintf(*w, "Loading datas:\n")
	for _, f := range reader.File {
		fileName := strings.SplitN(f.Name, "/", 2)[1]
		if len(fileName) == 0 || f.FileInfo().IsDir() {
			continue
		}
		if strings.HasPrefix(fileName, "data") {
			rc, err := f.Open()
			defer rc.Close()
			if err != nil {
				http.Error(*w, err.Error(), http.StatusInternalServerError)
				continue
			}
			fmt.Fprintf(*w, "%s used as data.\n", fileName)
			entities, err := loadCSV(rc)
			if err != nil {
				http.Error(*w, err.Error(), http.StatusInternalServerError)
				continue
			}
			println("Data:", fileName)
			var dataName = fileName[5 : len(fileName)-4]
			newData[dataName] = entities
			continue
		}
	}
	return &newData
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

	newData := loadDatas(reader, &w)

	for _, f := range reader.File {

		fileName := strings.SplitN(f.Name, "/", 2)[1]

		fmt.Fprintf(w, "Importing '%s':", fileName)
		if len(fileName) == 0 || f.FileInfo().IsDir() || strings.HasPrefix(fileName, "data") {
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
	_, err = datastore.Put(c, datastore.NewKey(c, "Data", "default", 0, nil), &dataStore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

/*
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

func loadGAEDatas(c appengine.Context) (DataEntities, error) {
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

func transformHandler(w http.ResponseWriter, r *http.Request) {
	// url https://github.com/olostan/test-website/zipball/master
	c := appengine.NewContext(r)
	datas, err := loadGAEDatas(c);
	fmt.Fprintln(w,"Transforming...");
	err = transformTemplates(c,&datas);
	if err!=nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	fmt.Fprintln(w,"Transformed");

}

func transformTemplates(c appengine.Context,datas *DataEntities) error {
	println("Transforming templates:");
	templates,err := loadTemplates(c);
	if err!=nil { return err;}
	q := datastore.NewQuery("Content").
	Filter("BlobKey =", "")

	var contents []Content
	if keys, err := q.GetAll(c, &contents); err != nil {
		return err
	} else {
		println("Loaded contents",len(contents))
		for i := range(contents) {
			var k *datastore.Key = keys[i]
			name := k.StringID()[1:]
			println("Transforming",name);
			//_,err = templates.New(name).Parse(string(contents[i].Data[:]))
			buf := new(bytes.Buffer)
			templates.ExecuteTemplate(buf,name,datas);
			if err != nil {
				return err
			}
				contents[i].Data = buf.Bytes();
			_, err := datastore.Put(c, k, &contents[i])
			if err !=nil {
				print(err);
				return err;
			}
		}
	}
	memcache.Flush(c);
	return nil;
}
*/
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
