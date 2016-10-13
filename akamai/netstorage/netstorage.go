package netstorage

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
	"fmt"
    "io"
    "io/ioutil"
    "math/rand"
	"net/http"
    "net/url"
    "os"
    "path"
    "strings"
    "time"
)

//
type Netstorage struct {
    hostname    string
    keyname     string
    key         string
    ssl         string
}

//
func NewNetstorage(hostname, keyname, key string, ssl bool) *Netstorage {
    if (hostname == "" && keyname == "" && key == "") {
        panic("[NetstorageError] You should input netstorage hostname, keyname and key all")
    }
    s := ""
    if ssl {
        s = "s"
    }
    return &Netstorage{hostname, keyname, key, s}
}

//
func _ifUploadAction(kwargs map[string]string) (*io.Reader, error) {
    var data io.Reader = nil
    if kwargs["action"] == "upload" {
        bArr, err := ioutil.ReadFile(kwargs["source"])
        if err != nil {
            return nil, err
        }

        data = bytes.NewReader(bArr)
    }
    return &data, nil
}

//
func _getBody(kwargs map[string]string, response *http.Response) (string, error) {
    var body []byte
    var err error
    if kwargs["action"] == "download" && response.StatusCode == 200 {
        localDestination := kwargs["destination"]

        if localDestination == "" {
            localDestination = path.Base(kwargs["path"]) 
        } else if s, err := os.Stat(localDestination); err == nil && s.IsDir() {
            localDestination = path.Join(localDestination, path.Base(kwargs["path"]))
        }

        out, err := os.Create(localDestination)
        if err != nil {
            return "", err
        }
        defer out.Close()

        if _, err := io.Copy(out, response.Body); err != nil {
            return "", err
        }
        body = []byte("Download done")
    } else {
        body, err = ioutil.ReadAll(response.Body)
        if err != nil {
            return "", err
        }
    }

    return string(body), nil
}

//
func (ns *Netstorage) _request(kwargs map[string]string) (*http.Response, string, error) {
    var err error

    nsPath := kwargs["path"]
    if u, err := url.Parse(nsPath); strings.HasPrefix(nsPath, "/") && err == nil {
        nsPath = u.RequestURI()
    } else {
        return nil, "", fmt.Errorf("[Netstorage Error] Invalid netstorage path: %s", nsPath)
    }

    acsAction := fmt.Sprintf("version=1&action=%s", kwargs["action"])
    acsAuthData := fmt.Sprintf("5, 0.0.0.0, 0.0.0.0, %d, %d, %s",
                                    time.Now().Unix(),
                                    rand.Intn(100000),
                                    ns.keyname)

    signString := fmt.Sprintf("%s\nx-akamai-acs-action:%s\n", nsPath, acsAction)
    mac := hmac.New(sha256.New, []byte(ns.key))
    mac.Write([]byte(acsAuthData + signString))
    acsAuthSign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

    data, err := _ifUploadAction(kwargs)
    if err != nil {
        return nil, "", err
    }

    request, err := http.NewRequest(kwargs["method"], 
        fmt.Sprintf("http%s://%s%s", ns.ssl, ns.hostname, nsPath), *data)
    
    if err != nil {
		return nil, "", err
	}

    request.Header.Add("X-Akamai-ACS-Action", acsAction)
    request.Header.Add("X-Akamai-ACS-Auth-Data", acsAuthData)
    request.Header.Add("X-Akamai-ACS-Auth-Sign", acsAuthSign)
    request.Header.Add("Accept-Encoding", "identity")
    request.Header.Add("User-Agent", "NetStorageKit-Golang")

    client := &http.Client{}
    response, err := client.Do(request)
    
    if err != nil {
		return nil, "", err
	}
    
    defer response.Body.Close()
    body, err := _getBody(kwargs, response)
    
    return response, body, err
}

//
func (ns *Netstorage) Dir(nsPath string) (*http.Response, string, error) {
    return ns._request(map[string]string{
        "action": "dir&format=xml",
        "method": "GET",
        "path": nsPath,
    })
}

//
func (ns *Netstorage) Download(path ...string) (*http.Response, string, error) {
    ns_source := path[0]
    if strings.HasSuffix(ns_source, "/") {
        return nil, "", fmt.Errorf("[NetstorageError] Nestorage download path shouldn't be a directory: %s", ns_source)
    }

    localDestination := ""
    if len(path) >= 2 {
        localDestination = path[1]
    }

    return ns._request(map[string]string{
        "action": "download",
        "method": "GET",
        "path": ns_source,
        "destination": localDestination,
    })
}

//
func (ns *Netstorage) Du(nsPath string) (*http.Response, string, error) {
    return ns._request(map[string]string{
        "action": "du&format=xml",
        "method": "GET",
        "path": nsPath,
    })
}

//
func (ns *Netstorage) Stat(nsPath string) (*http.Response, string, error) {
    return ns._request(map[string]string{
        "action": "stat&format=xml",
        "method": "GET",
        "path": nsPath,
    })
}

//
func (ns *Netstorage) Mkdir(nsPath string) (*http.Response, string, error) {
    return ns._request(map[string]string{
        "action": "mkdir",
        "method": "POST",
        "path": nsPath,
    })
}

//
func (ns *Netstorage) Rmdir(nsPath string) (*http.Response, string, error) {
    return ns._request(map[string]string{
        "action": "rmdir",
        "method": "POST",
        "path": nsPath,
    })
}

//
func (ns *Netstorage) Mtime(nsPath string, mtime int64) (*http.Response, string, error) {
    return ns._request(map[string]string{
        "action": fmt.Sprintf("mtime&format=xml&mtime=%d", mtime),
        "method": "POST",
        "path": nsPath,
    })
}

//
func (ns *Netstorage) Delete(nsPath string) (*http.Response, string, error) {
    return ns._request(map[string]string{
        "action": "delete",
        "method": "POST",
        "path": nsPath,
    })
}

//
func (ns *Netstorage) Quick_delete(nsPath string) (*http.Response, string, error) {
    return ns._request(map[string]string{
        "action": "quick-delete&quick-delete=imreallyreallysure",
        "method": "POST",
        "path": nsPath,
    })
}

//
func (ns *Netstorage) Rename(nsTarget, nsDestination string) (*http.Response, string, error) {
    return ns._request(map[string]string{
        "action": "rename&destination=" + url.QueryEscape(nsDestination),
        "method": "POST",
        "path": nsTarget,
    })
}

//
func (ns *Netstorage) Symlink(nsTarget, nsDestination string) (*http.Response, string, error) {
    return ns._request(map[string]string{
        "action": "symlink&target=" + url.QueryEscape(nsTarget),
        "method": "POST",
        "path": nsDestination,
    })
}

//
func (ns *Netstorage) Upload(localSource, nsDestination string) (*http.Response, string, error) {
    s, err := os.Stat(localSource)

    if err != nil {
        return nil, "", err
    }   

    if s.Mode().IsRegular() {    
        if strings.HasSuffix(nsDestination, "/") {
            nsDestination = nsDestination + path.Base(localSource)
        }
    } else {
        return nil, "", fmt.Errorf("[NetstorageError] You should upload a file, not %s", localSource)
    }
    
    return ns._request(map[string]string{
        "action": "upload",
        "method": "PUT",
        "source": localSource,
        "path": nsDestination,
    })
}