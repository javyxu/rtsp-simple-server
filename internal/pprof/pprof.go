package pprof

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/aler9/rtsp-simple-server/internal/conf"

	// start pprof
	_ "net/http/pprof"
)

const (
	address = ":9999"
)

// Parent is implemented by program.
type Parent interface {
	Log(string, ...interface{})
}

// Pprof is a performance metrics exporter.
type Pprof struct {
	listener net.Listener
	server   *http.Server
	confPath string
	conf     *conf.Conf
}

// New allocates a Pprof.
func New(cfgpath string, conf *conf.Conf, parent Parent) (*Pprof, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	pp := &Pprof{
		listener: listener,
		confPath: cfgpath,
		conf:     conf,
	}

	http.HandleFunc("/rtspManager/addRTSPUrl", pp.addrtspurl)
	http.HandleFunc("/rtspManager/getRTSPUrls", pp.getrtspurls)
	http.HandleFunc("/rtspManager/deleteRTSPUrl", pp.deletertspurl)
	pp.server = &http.Server{
		Handler: http.DefaultServeMux,
	}

	parent.Log("[pprof] opened on " + address)

	go pp.run()
	return pp, nil
}

// Close closes a Pprof.
func (pp *Pprof) Close() {
	pp.server.Shutdown(context.Background())
}

func (pp *Pprof) run() {
	err := pp.server.Serve(pp.listener)
	if err != http.ErrServerClosed {
		panic(err)
	}
}

// Resp is response for http
type Resp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func (pp *Pprof) addrtspurl(writer http.ResponseWriter, request *http.Request) {

	d := json.NewDecoder(request.Body)
	t := struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}{}
	d.Decode(&t)

	newURL := make(map[string]string)
	newURL[t.Name] = fmt.Sprintf("rtsp://%s:%s/%s", strings.Split(request.Host, ":")[0], strconv.Itoa(pp.conf.RtspPort), t.Name)
	if pp.conf.NameIsExist(t.Name) {
		var result Resp
		result.Code = 100002
		result.Msg = "rtsp url already exist"
		result.Data = newURL
		if err := json.NewEncoder(writer).Encode(result); err != nil {
			log.Fatal(err)
		}
	} else {
		var cmd string
		sysType := runtime.GOOS
		if sysType == "darwin" {
			cmd = "gsed"
		} else {
			cmd = "sed"
		}
		cmdreg := fmt.Sprintf("/paths:/a\\  %s:\\n    source: %s\\n    sourceOnDemand: yes", t.Name, t.URL)
		srcfile := pp.confPath
		command := exec.Command(cmd, "-i", cmdreg, srcfile)
		_, err := command.CombinedOutput()

		var result Resp
		if err != nil {
			result.Code = 100001
			result.Msg = "添加失败"
			result.Data = ""
			if err := json.NewEncoder(writer).Encode(result); err != nil {
				log.Fatal(err)
			}
		} else {
			result.Code = 100000
			result.Msg = "添加成功"
			result.Data = newURL
			if err := json.NewEncoder(writer).Encode(result); err != nil {
				log.Fatal(err)
			}
		}
	}
}

type urlStruct struct {
	Name      string `json:"name"`
	TargetURL string `json:"targeturl"`
	SourceURL string `json:"sourceurl"`
}

func (pp *Pprof) getrtspurls(writer http.ResponseWriter, request *http.Request) {

	var urls []urlStruct

	for name, pconf := range pp.conf.Paths {
		url := urlStruct{
			Name:      name,
			SourceURL: pconf.Source,
			TargetURL: fmt.Sprintf("rtsp://%s:%s/%s", strings.Split(request.Host, ":")[0], strconv.Itoa(pp.conf.RtspPort), name),
		}
		urls = append(urls, url)
	}

	var result Resp
	result.Code = 100000
	result.Msg = "获取摄像头列表成功"
	result.Data = urls
	if err := json.NewEncoder(writer).Encode(result); err != nil {
		log.Fatal(err)
	}
}

func (pp *Pprof) deletertspurl(writer http.ResponseWriter, request *http.Request) {

	d := json.NewDecoder(request.Body)
	t := struct {
		Name string `json:"name"`
	}{}
	d.Decode(&t)

	var cmd string
	sysType := runtime.GOOS
	if sysType == "darwin" {
		cmd = "gsed"
	} else {
		cmd = "sed"
	}
	cmdreg := fmt.Sprintf("/%s/=", t.Name)
	srcfile := pp.confPath
	command := exec.Command(cmd, "-n", cmdreg, srcfile)
	colnum, _ := command.CombinedOutput()
	colnumInt, _ := strconv.Atoi(strings.Replace(string(colnum), "\n", "", -1))

	var err error
	if colnumInt > 0 {
		cmdreg = fmt.Sprintf("%d,%dd", colnumInt, colnumInt+2)
		command = exec.Command(cmd, "-i", cmdreg, srcfile)
		_, err = command.CombinedOutput()
	} else {
		err = nil
	}

	var result Resp
	if err != nil {
		result.Code = 100001
		result.Msg = "删除失败"
		result.Data = ""
		if err := json.NewEncoder(writer).Encode(result); err != nil {
			log.Fatal(err)
		}
	} else {
		result.Code = 100000
		result.Msg = "删除成功"
		result.Data = ""
		if err := json.NewEncoder(writer).Encode(result); err != nil {
			log.Fatal(err)
		}
	}
}
