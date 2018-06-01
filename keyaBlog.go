package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

// DLserver 下载控制器
type DLserver struct {
	WG    sync.WaitGroup
	Gonum chan string
}

// Blogs 博客结构
type Blogs struct {
	title  string
	author string
	date   string
	text   string
	imgs   []string
}

// getCurrentDirectory 获取当前路径
func getCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

// PathExists 判断文件夹是否存在
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// PurifyBlog 提取文本
func PurifyBlog(str string) (newStr string) {
	regTag := regexp.MustCompile(`<[^img].*?>|<img src="|" alt=".*?"|id=".*?"/>|<.*?>|^[\s]*`)
	newStr = regTag.ReplaceAllString(str, "\r\n")
	return
}

// RemoveBlank 移除空白字符
func RemoveBlank(str string) (newStr string) {
	regSpa := regexp.MustCompile(`^[\s]+|[\s]+$`)
	newStr = regSpa.ReplaceAllString(str, "")
	return
}

// MergeLine 合并换行符
func MergeLine(str string) (newStr string) {
	regSpa := regexp.MustCompile(`\s{2,}`)
	newStr = regSpa.ReplaceAllString(str, "\r\n")
	return
}

// ImgURLGet 获取图片地址
func ImgURLGet(blogText string) (imgs []string) {
	regJpg := regexp.MustCompile(`http:.*jpg`)
	imgs = regJpg.FindAllString(blogText, -1)
	return
}

// SaveToText 保存内容
func SaveToText(blogRaw *goquery.Document, dl *DLserver, thread int) {
	blogInfo, savename := FormatInfo(blogRaw)
	// 创建保存的目录 文件名
	blogMain := blogInfo.title + "\r\n" + blogInfo.date + "\r\n" + blogInfo.text
	imgs := blogInfo.imgs
	mainFolder := getCurrentDirectory() + "//" + blogInfo.author
	mainExist, _ := pathExists(mainFolder)
	if !mainExist {
		os.Mkdir(mainFolder, os.ModePerm)
	}
	subFolder := mainFolder + "//" + savename
	subExist, _ := pathExists(subFolder)
	if !subExist {
		os.Mkdir(subFolder, os.ModePerm)
	}
	filename := subFolder + "//" + savename + ".txt"

	// 启用多核
	runtime.GOMAXPROCS(runtime.NumCPU())
	// 用channel阻塞控制goroutine的数量
	dl.Gonum = make(chan string, thread)
	// 当前任务数量
	tasks := len(imgs)
	dl.WG.Add(tasks)
	log.Printf("Total [%d]\n", tasks)
	// 执行下载
	for _, img := range imgs {
		dl.Gonum <- img
		go downloadEngine(img, subFolder, dl)
	}
	dl.WG.Wait()
	ioutil.WriteFile(filename, []byte(blogMain), 0644)
	log.Printf("All is down")
}

// downloadEngine 下载函数
func downloadEngine(img string, path string, dl *DLserver) {
	client := &http.Client{}
	urlCut := strings.Split(img, "/")
	filename := urlCut[len(urlCut)-1]
	req, _ := http.NewRequest("GET", img, nil)
	log.Printf("[%s is Start..]\n", filename)
	req.Header.Add("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1")
	res, _ := client.Do(req)
	defer res.Body.Close()
	savePath := path + "//" + filename
	file, _ := os.Create(savePath)
	io.Copy(file, res.Body)
	log.Printf("---- %s is done\n", filename)
	dl.WG.Done()
	// 每个goroutine完成后取出
	<-dl.Gonum
}

// FormatInfo 格式化输出内容
func FormatInfo(blogRaw *goquery.Document) (BlogInfo Blogs, Date string) {
	doc := blogRaw
	Newbody := doc.Find("article")
	BlogCont, _ := Newbody.Find(".box-article").Html()

	BlogInfo.title = RemoveBlank(Newbody.Find(".box-ttl h3").Text())
	BlogInfo.author = RemoveBlank(Newbody.Find(".name").Text())
	BlogInfo.date = RemoveBlank(Newbody.Find(".box-bottom li").Text())

	PurifyText := PurifyBlog(BlogCont)
	BlogInfo.text = MergeLine(PurifyText)
	BlogInfo.imgs = ImgURLGet(PurifyText)

	// 格式化时间作为目录名
	BlogDateFormat := strings.Replace(RemoveBlank(BlogInfo.date), "/", "-", -1)
	BlogDateFormat = strings.Replace(RemoveBlank(BlogDateFormat), " ", "-", -1)
	Date = strings.Replace(RemoveBlank(BlogDateFormat), ":", "-", -1)
	return
}

// GetBlogRaw 获取原始内容
func GetBlogRaw(url string) *goquery.Document {
	log.Println("Get Blog Info...")
	// 自定义UA的Get请求
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1")
	res, _ := client.Do(req)
	defer res.Body.Close()
	// 返回goquery的结构
	body, _ := goquery.NewDocumentFromReader(res.Body)
	return body
}

func main() {
	fmt.Println("http://www.keyakizaka46.com/s/k46o/diary/detail/15117?ima=0000&cd=member")
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Url: ")
	data, _, _ := reader.ReadLine()
	url := string(data)
	dl := new(DLserver)
	blogRaw := GetBlogRaw(url)
	SaveToText(blogRaw, dl, 4)
}
