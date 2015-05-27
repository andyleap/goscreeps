// goscreep project main.go
package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"fmt"
	"net/http"

	"gopkg.in/fsnotify.v1"
)

type Settings struct {
	Username string
	Password string
	Source   string
}

var settings Settings

type ApiData struct {
	Modules map[string]string `json:"modules"`
}

var Send = make(chan bool, 1)

func main() {
	buf, _ := ioutil.ReadFile("settings.json")
	json.Unmarshal(buf, &settings)
	_, err := os.Stat(settings.Source)
	if os.IsNotExist(err) {
		PullCode()
	} else {
		PushCode()
	}
	watcher, _ := fsnotify.NewWatcher()
	watcher.Add(settings.Source)
	filepath.Walk(settings.Source, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			watcher.Add(path)
		}
		return nil
	})
	go func() {
		for {
			<-Send
			PushCode()
			time.Sleep(time.Second * 5)
		}
	}()
	for {
		select {
		case event := <-watcher.Events:
			if event.Op == fsnotify.Create {
				added, _ := os.Stat(event.Name)
				if added.IsDir() {
					watcher.Add(event.Name)
				}
			}
			select {
			case Send <- true:
			default:
			}
		case err := <-watcher.Errors:
			fmt.Println("error:", err)
		}
	}
}

func PullCode() {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://screeps.com/api/user/code", nil)
	req.SetBasicAuth(settings.Username, settings.Password)
	resp, _ := client.Do(req)
	buf, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	api := &ApiData{}
	json.Unmarshal(buf, &api)
	for name, module := range api.Modules {
		name = filepath.FromSlash(name)
		dir, _ := filepath.Split(name)
		os.MkdirAll(path.Join(settings.Source, dir), 0666)
		ioutil.WriteFile(path.Join(settings.Source, name+".js"), []byte(module), 0666)
	}
}

func PushCode() {
	client := &http.Client{}

	files := &ApiData{Modules: make(map[string]string)}

	filepath.Walk(settings.Source, func(path string, info os.FileInfo, err error) error {
		name := strings.TrimPrefix(path, settings.Source)
		name = strings.TrimPrefix(name, string(os.PathSeparator))
		name = filepath.ToSlash(name)
		name = strings.TrimSuffix(name, ".js")
		if !info.IsDir() {
			data, _ := ioutil.ReadFile(path)
			files.Modules[name] = string(data)
		}
		return nil
	})
	body, _ := json.Marshal(files)
	fmt.Println(string(body))
	req, _ := http.NewRequest("POST", "https://screeps.com/api/user/code", bytes.NewReader(body))
	req.SetBasicAuth(settings.Username, settings.Password)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.ContentLength = int64(len(body))
	resp, _ := client.Do(req)
	buf, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println(string(buf))
}
