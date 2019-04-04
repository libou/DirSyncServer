package main

import (
	"DirSyncSystemServer/logFile"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

//type Arg struct{
//	FilePathAll []string
//}
//
//type Reply struct {
//	FilePathAll []string
//}

var port string
var dirPath string

type ClientArg struct {
	FileList    map[string][]byte `json:"file_list"`
}

type ClientReply struct{
	RemoveFile   []string `json:"remove_file"`
	DownloadFile []string `json:"download_file"`
}

func init(){
	logFile.InitLog()
	port = beego.AppConfig.DefaultString("server::listen_port","4321")
	dirPath = beego.AppConfig.DefaultString("dir::path","/syncDir")
}


func main(){
	listener,err := net.Listen("tcp",":" + port)
	if err != nil {
		logs.Error("[LISTENER ERROR] [%s]\r",err)
		fmt.Println(err)
		return
	}
	defer listener.Close()

	for {
		conn,err := listener.Accept()
		if err != nil {
			logs.Error("[LISTEN ACCEPT ERROR] [%s]\r",err)
			fmt.Println(err)
			return
		}
		go func(conn net.Conn){
			defer conn.Close()

			buf := make([]byte,1024)
			size,err := conn.Read(buf)
			if err != nil {
				logs.Error("[READ ERROR] [%s]\r",err)
				fmt.Println(err)
				return
			}
			oper := string(buf[:size])
			switch oper {
				case "create":
					ReadFile(conn)
				case "write":
					ReadFile(conn)
				case "remove":
					RemoveFile(conn)
				case "rename":
					RemoveFile(conn)
				case "sync":
					Sync(conn)
				case "download":
					DownloadFile(conn)
			}

		}(conn)
	}
}



func Sync(conn net.Conn) {
	_,err := conn.Write([]byte("ok"))
	if err != nil {
		fmt.Println(err)
		logs.Error("[SYNC WRITE ERROR] [%s]\r",err)
		return
	}

	buff := make([]byte,1024*4)
	var sumBuff string = ""
	//获取客户端目录文件列表
	for {
		size,err := conn.Read(buff)
		if err != nil && err != io.EOF {
			logs.Error("[CONN READ ERROR] [%s]\r",err)
			fmt.Println(err)
		}
		if string(buff[size-3:size]) == "end" {
			sumBuff = sumBuff + string(buff[:size-3])
			break
		}else {
			sumBuff = sumBuff + string(buff[:size])
		}

	}
	ca := ClientArg{}
	err = json.Unmarshal([]byte(sumBuff),&ca)
	if err != nil {
		logs.Error("[SYNC JSON DISCODE ERROR] [%s]\r",ca)
		fmt.Println(err)
		return
	}
	//检测文件
	cr := FindNeededSyncFile(ca)
	//向客户端发送反馈
	reply,err := json.Marshal(cr)
	if err != nil {
		logs.Error("[SYNC JSON ENCODE ERROR] [%s]\r",ca)
		fmt.Println(err)
		return
	}
	lenOfReply := len(reply)
	n := lenOfReply/cap(buff)
	sum := 0
	for i := 0;i <= n;i++ {
		if i != n {
			_,err = conn.Write(reply[sum:sum+1024*4])
		}else {
			_,err = conn.Write(reply[sum:])
		}
		if err != nil {
			fmt.Println("同步出错:",err)
			logs.Error("[SYNC WRITE ERROR] [%s]\r",reply)
			return
		}
		sum = sum +1024*4
	}

	_,err = conn.Write([]byte("end"))
	if err != nil {
		fmt.Println("同步出错:",err)
		logs.Error("[SYNC WRITE ERROR] [%s]\r",reply)
		return
	}
	//_,err = conn.Write(reply)
	//if err != nil {
	//	fmt.Println("同步出错:",err)
	//	logs.Error("[SYNC WRITE ERROR] [%s]\r",reply)
	//	return
	//}

}

func FindNeededSyncFile(ca ClientArg) ClientReply{
	cr := ClientReply{}
	serverFile := make(map[string][]byte)

	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if ! info.IsDir(){
			path, err := filepath.Abs(path)
			if err != nil {
				return err
			}

			f,err := os.Open(path)
			if err != nil {
				fmt.Println("md5 open ",err)
				logs.Error("[MD5 OPEN ERROR] [%s]\r",path)
				return err
			}
			md5hash := md5.New()
			if _,err := io.Copy(md5hash,f);err == nil{
				path = strings.TrimPrefix(path,dirPath+"/")
				serverFile[path] = md5hash.Sum(nil)
			}else {
				logs.Error("[md5 ERROR] [%s]\r",path)
				fmt.Println("md5 error",err)
			}
			f.Close()

			//path = strings.TrimPrefix(path,dirPath+"/")
			//serverFile[path] = info.ModTime()
		}
		return nil
	})

	//确定待删除文件列表
	for cfp,_ := range ca.FileList {
		//服务器上不存在该文件
		if _,ok := serverFile[cfp];!ok{
			//需要删除，放入待删除列表
			cr.RemoveFile = append(cr.RemoveFile,cfp)
		}
	}

	//确定待下载文件列表
	for sfp,sut := range serverFile {
		//客户端存在该文件
		if cut,ok := ca.FileList[sfp];ok{
			//判断是否需要更新文件
			//timeInterval := sut.Sub(cut)
			////fmt.Println(timeInterval.Seconds())
			//if timeInterval.Seconds() > 300 {
			//	cr.DownloadFile = append(cr.DownloadFile, sfp)
			//}
			if string(sut) != string(cut) {
				cr.DownloadFile = append(cr.DownloadFile,sfp)
			}
		}else {
			//客户端不存在该文件，放入下载列表
			cr.DownloadFile = append(cr.DownloadFile,sfp)
		}
	}

	logs.Info("[同步文件列表] [%s]\r",cr)
	return cr
}

func DownloadFile(conn net.Conn) {
	_,err := conn.Write([]byte("ok"))
	if err != nil {
		fmt.Println(err)
		logs.Error("[CONN WRITE ERROR] [%s]\r",err)
		return
	}

	buff := make([]byte,1024*4)
	size,err := conn.Read(buff)
	if err != nil && err != io.EOF{
		fmt.Println(err)
		logs.Error("[DOWNLOAD READ ERROR] [%s]\r",err)
		return
	}
	fp := string(buff[:size])

	file,err := os.Open(filepath.Join(dirPath,fp))
	if err != nil{
		fmt.Println(err)
		logs.Error("[OPEN FILE ERROR] [%s]\r",filepath.Join(dirPath,fp))
		return
	}
	defer file.Close()

	for {
		size, err := file.Read(buff)
		if err != nil {
			if err == io.EOF {
				fmt.Println("下载完毕：", fp)
				logs.Info("[下载完成] [%s]\r",fp)
			} else {
				logs.Error("[下载出错] [%s] [%s]\r",fp,err)
				fmt.Println(err)
				return
			}
			return
		}
		_,err = conn.Write(buff[:size])
		if err != nil {
			fmt.Println("下载出错：",err)
			logs.Error("[下载出错] [%s] [%s]\r",fp,err)
		}
	}
}

func RemoveFile(conn net.Conn){
	_,err :=conn.Write([]byte("ok"))
	if err != nil {
		fmt.Println(err)
		logs.Error("[REMOVE WRITE ERROR] [%s]\r",err)
		return
	}

	buf := make([]byte,1024*4)
	size,err := conn.Read(buf)
	if err != nil && err != io.EOF {
		fmt.Println(err)
		logs.Error("[REMOVE READ ERROR] [%s]\r",err)
		return
	}
	filePath := string(buf[:size])
	err = os.RemoveAll(filepath.Join(dirPath,filePath))
	if err != nil {
		fmt.Println(err)
		logs.Error("[删除错误] [%s]\r",err)
		return
	}

}

func ReadFile(conn net.Conn){
	//向客户端发送确认开始上传的消息
	_,err :=conn.Write([]byte("ok"))
	if err != nil {
		fmt.Println(err)
		logs.Error("[CONN WRITE ERROR] [%s]\r",err)
		return
	}

	//接受上传文件名
	buf := make([]byte,1024*4)
	size,err := conn.Read(buf)
	if err != nil {
		logs.Error("[CONN READ ERROR] [%s]\r",err)
		fmt.Println(err)
		return
	}
	filePath := string(buf[:size])
	fileName := filePath[strings.LastIndex(filePath,"/")+1:]
	filePath = filePath[:strings.LastIndex(filePath,"/")+1]

	err = os.MkdirAll(filepath.Join(dirPath,filePath),os.ModePerm)
	if err != nil {
		fmt.Println(err)
		logs.Error("[MKDIR ERROR] [%s]\r",err)
		return
	}
	file,err := os.Create(filepath.Join(dirPath,filePath,fileName))
	defer file.Close()
	if err != nil {
		logs.Error("[FILE CREATE ERROR] [%s]\r",filepath.Join(dirPath,filePath,fileName))
		fmt.Println(err)
		return
	}
	_,err = conn.Write([]byte("ok"))
	if err != nil {
		fmt.Println(err)
		logs.Error("[CONN WRITE ERROR] [%s]\r",err)
		return
	}

	for {
		size,err := conn.Read(buf)
		if err != nil{
			if err == io.EOF{
				fmt.Println("上传完成：",fileName)
				logs.Info("[上传完成] [%s]\r",fileName)
			}else{
				fmt.Println("上传失败：",err)
				logs.Error("[上传失败] [%s]\r",fileName)
				return
			}
			return
		}
		_,err = file.Write(buf[:size])
		if err != nil {
			fmt.Println("上传失败：",err)
			logs.Error("[上传失败] [%s]\r",fileName)
			return
		}
	}

}
