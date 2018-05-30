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
	"strings"
	"runtime"
	"github.com/PuerkitoBio/goquery"
)

var imgURL chan string
var quit chan bool

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

// SaveToText 合并换行符
func SaveToText(blogRaw *goquery.Document) {
	blogMain, imgs, folder, savename := FormatInfo(blogRaw)
	mainFolder := getCurrentDirectory() + "//" + folder
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
	runtime.GOMAXPROCS(4)
	imgURL = make(chan string)
	quit = make(chan bool)
	go downloadEngine(imgURL, quit, subFolder)
	for _, i := range imgs {
		imgURL <- i
	}
	quit <- false
	ioutil.WriteFile(filename, []byte(blogMain), 0644)
}

// downloadEngine
func downloadEngine(imgs chan string, quit chan bool, path string) {
	for {
		select {
		case img := <-imgs:
			client := &http.Client{}
			req, _ := http.NewRequest("GET", img, nil)
			req.Header.Add("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1")
			res, _ := client.Do(req)
			defer res.Body.Close()
			urlCut := strings.Split(img, "/")
			filename := urlCut[len(urlCut)-1]
			savePath := path + "//" + filename
			file, _ := os.Create(savePath)
			io.Copy(file, res.Body)
		case <-quit:
			break
		}
	}
}

// FormatInfo 格式化输出内容
func FormatInfo(blogRaw *goquery.Document) (Blog string, imgs []string, folder string, savename string) {
	doc := blogRaw
	Newbody := doc.Find("article")
	BlogTitle := Newbody.Find(".box-ttl h3").Text()
	// fmt.Print(RemoveBlank(BlogTitle))
	Blog = Blog + RemoveBlank(BlogTitle) + "\r\n"
	BlogAuthor := Newbody.Find(".name").Text()
	// fmt.Println(RemoveBlank(BlogAuthor))
	// Blog = Blog + RemoveBlank(BlogAuthor) + "\r\n"
	BlogDate := Newbody.Find(".box-bottom li").Text()
	// fmt.Println(RemoveBlank(BlogDate))
	BlogDateFormat := strings.Replace(RemoveBlank(BlogDate), "/", "-", -1)
	BlogDateFormat = strings.Replace(RemoveBlank(BlogDateFormat), " ", "-", -1)
	BlogDateFormat = strings.Replace(RemoveBlank(BlogDateFormat), ":", "-", -1)
	Blog = Blog + RemoveBlank(BlogDate) + "\r\n"
	// fmt.Println(Blog)
	BlogCont, _ := Newbody.Find(".box-article").Html()
	BlogCont = PurifyBlog(BlogCont)
	// fmt.Println(BlogCont)
	Blog = Blog + MergeLine(BlogCont)
	imgs = ImgURLGet(Blog)
	folder = RemoveBlank(BlogAuthor)
	savename = BlogDateFormat
	return
}

// GetBlogRaw 获取原始内容
func GetBlogRaw(url string) *goquery.Document {
	fmt.Println("Get Blog Info...")
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1")
	res, _ := client.Do(req)
	defer res.Body.Close()
	body, _ := goquery.NewDocumentFromReader(res.Body)
	return body
}

func main() {
	fmt.Println("http://www.keyakizaka46.com/s/k46o/diary/detail/15117?ima=0000&cd=member")
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Url: ")
	data, _, _ := reader.ReadLine()
	url := string(data)
	blogRaw := GetBlogRaw(url)
	SaveToText(blogRaw)
}

