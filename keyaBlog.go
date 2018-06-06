package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

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

// BlogMode 博客地址结构
type BlogMode struct {
	mode   string
	url    string
	number []string
	tag    string
	status bool
}

// request 统一请求结构
func request(method string, url string, body io.Reader) (*http.Response, error) {
	client := http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest(method, url, body)
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/62.0.3202.94 Safari/537.36")
	return client.Do(req)
}

// getCurrentDirectory 获取当前路径
func getCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

// pathExists 判断文件夹是否存在
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

// urlCheck 检查地址有效性
func urlCheck(url string) (valid string, ok bool) {
	if strings.Contains(url, "http://www.keyakizaka46.com/s/k46o") {
		valid = url
		ok = true
	} else {
		valid = ""
		ok = false
	}
	return
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

// pageURLs 从每页获取单篇博客的地址
func pageURLs(url string) (urls []string) {
	blogRaw := GetBlogRaw(url)
	var urlsTmp []string
	blogRaw.Find("article .box-bottom").Each(func(i int, s *goquery.Selection) {
		u, _ := s.Find("a").Attr("href")
		u = "http://www.keyakizaka46.com" + u
		urlsTmp = append(urlsTmp, u)
	})
	urls = urlsTmp
	return
}

// MultiPageURLs 获取指定页数的所有地址
func MultiPageURLs(tag string, numArray []string, url string) (urls []string) {
	page := getCurrentPage(url)
	number, _ := strconv.ParseInt(numArray[0], 10, 64)
	num := int(number)
	var pageURLTmp []string
	for num >= 0 {
		u := replaceURL(url, page, false)
		pageURLTmp = append(pageURLTmp, u)
		if tag == "puls" {
			page++
		} else {
			page--
		}
		num--
	}
	for _, pageU := range pageURLTmp {
		url := pageURLs(pageU)
		urls = append(urls, url...)
	}
	return
}

// singlePageTurn 查找翻页标签
func singlePageTurn(tag string, url string) (turl string, isExist bool) {
	if tag == "plus" {
		blogRaw := GetBlogRaw(url)
		nextURL, _ := blogRaw.Find(".btn-next").Find("a").Attr("href")
		if len(nextURL) > 0 {
			turl = "http://www.keyakizaka46.com" + nextURL
			isExist = true
		} else {
			turl = ""
			isExist = false
		}
	} else if tag == "minus" {
		blogRaw := GetBlogRaw(url)
		prevURL, _ := blogRaw.Find(".btn-prev").Find("a").Attr("href")
		if len(prevURL) > 0 {
			turl = "http://www.keyakizaka46.com" + prevURL
			isExist = true
		} else {
			turl = ""
			isExist = false
		}
	} else {
		turl = ""
		isExist = false
	}
	return
}

// singleURLs 自动翻页
func singleURLs(tag string, numArray []string, url string) (urls []string) {
	urls = append(urls, url)
	number, _ := strconv.ParseInt(numArray[0], 10, 64)
	num := int(number)
	var urltmp string
	urltmp = url
	for num > 0 {
		purl, isExist := singlePageTurn(tag, urltmp)
		if isExist {
			urls = append(urls, purl)
			urltmp = purl
		}
		num--
	}
	return urls
}

// numAnalysis 获取博客的数量
func numAnalysis(num string) (tag string, number []string) {
	regPlus := regexp.MustCompile(`^\+[0-9]+$`)
	regMinus := regexp.MustCompile(`^\-[0-9]+$`)
	regRange := regexp.MustCompile(`^[0-9]+\-[0-9]+$`)
	if num == "1" {
		number = []string{num}
		tag = "one"
	} else if regPlus.MatchString(num) {
		numParts := strings.Split(num, "")
		tag = "plus"
		number = []string{numParts[1]}
	} else if regMinus.MatchString(num) {
		numParts := strings.Split(num, "")
		tag = "minus"
		number = []string{numParts[1]}
	} else if regRange.MatchString(num) {
		numParts := strings.Split(num, "")
		if numParts[0] < numParts[2] {
			tag = "range"
			sta, _ := strconv.ParseInt(numParts[0], 10, 16)
			end, _ := strconv.ParseInt(numParts[2], 10, 16)
			numTemp := []string{}
			for i := sta; i <= end; i++ {
				newi := strconv.FormatInt(i, 10)
				numTemp = append(numTemp, newi)
			}
			number = numTemp
		} else {
			number = []string{"None"}
			tag = "None"
		}
	} else {
		number = []string{"None"}
		tag = "None"
	}
	return
}

// getCurrentPage 获取当前页码
func getCurrentPage(url string) (page int) {
	regPage := regexp.MustCompile(`page=[0-9]+`)
	currentPageStr := regPage.FindAllString(url, -1)[0]
	pageStr := strings.Split(currentPageStr, "=")[1]
	page, _ = strconv.Atoi(pageStr)
	return
}

// replaceURL 生成所有页的地址
func replaceURL(url string, page int, flag bool) (nurl string) {
	regPage := regexp.MustCompile(`page=[0-9]+`)
	if flag {
		page = page - 1
	} else {
		page = page
	}
	newp := "page=" + strconv.Itoa(page)
	nurl = regPage.ReplaceAllString(url, newp)
	return
}

// GetBlogRaw 获取原始内容
func GetBlogRaw(url string) *goquery.Document {
	res, _ := request("GET", url, nil)
	defer res.Body.Close()
	// 返回goquery的结构
	body, _ := goquery.NewDocumentFromReader(res.Body)
	return body
}

// downloadEngine 下载函数
func downloadEngine(id int, img string, path string, dl *DLserver) {
	urlCut := strings.Split(img, "/")
	filename := urlCut[len(urlCut)-1]
	log.Printf("<Task %d> [%s is start..]\n", id, filename)
	res, _ := request("GET", img, nil)
	defer res.Body.Close()
	savePath := path + "//" + filename
	file, _ := os.Create(savePath)
	io.Copy(file, res.Body)
	log.Printf("<Task %d> [%s is done]\n", id, filename)
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

// SaveToText 保存内容
func SaveToText(id int, blogRaw *goquery.Document, dl *DLserver, thread int) {
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
	log.Printf("<Task %d> imgs [%d]\n", id, tasks)
	// 执行下载
	for _, img := range imgs {
		dl.Gonum <- img
		go downloadEngine(id, img, subFolder, dl)
	}
	dl.WG.Wait()
	ioutil.WriteFile(filename, []byte(blogMain), 0644)
	log.Printf("---- <Task %d> is all done ----", id)
}

// URLAnalysis 识别地址类型
func URLAnalysis(url string, num string) (BlogMode BlogMode) {
	if strings.Contains(url, "artist") {
		tag, number := numAnalysis(num)
		if tag != "None" {
			urlParts := strings.Split(url, "/")
			membercode := strings.Split(urlParts[6], "?")[0]
			urlReform := urlParts[0] + "/" + urlParts[1] + "/" + urlParts[2] + "/" + urlParts[3] + "/" + urlParts[4] + "/diary/member/list?ima=0000&ct="+ membercode +"&cd=member&page=0"
			BlogMode.mode = "page"
			BlogMode.url = urlReform
			BlogMode.number = number
			BlogMode.tag = tag
			BlogMode.status = true
		} else {
			BlogMode.status = false
		}
	} else if strings.Contains(url, "diary/member") {
		tag, number := numAnalysis(num)
		if tag != "None" {
			var urlReform string
			if strings.Contains(url, "page") {
				urlReform = url
			} else {
				urlReform = url + "&cd=member&page=0"
			}
			BlogMode.mode = "page"
			BlogMode.url = urlReform
			BlogMode.number = number
			BlogMode.tag = tag
			BlogMode.status = true
		} else {
			BlogMode.status = false
		}

	} else if strings.Contains(url, "diary/detail") {
		tag, number := numAnalysis(num)
		if tag != "None" {
			BlogMode.mode = "single"
			BlogMode.url = url
			BlogMode.number = number
			BlogMode.tag = tag
			BlogMode.status = true
		} else {
			BlogMode.status = false
		}
	} else {
		BlogMode.status = false
	}
	return
}

// BlogURLsGet 获取所有博客地址
func BlogURLsGet(BlogMode BlogMode) (urls []string, stat bool) {
	if BlogMode.status {
		url := BlogMode.url
		mode := BlogMode.mode
		tag := BlogMode.tag
		number := BlogMode.number
		if mode == "single" {
			stat = true
			if tag == "range" {
				stat = false
				fmt.Println("No range type.")
			} else if tag == "plus" {
				urls = singleURLs(tag, number, url)
			} else if tag == "minus" {
				urls = singleURLs(tag, number, url)
			} else {
				urls = []string{url}
			}
		} else if mode == "page" {
			stat = true
			if tag == "range" {
				var pageURLTmp []string
				for _, i := range number {
					p, _ := strconv.Atoi(i)
					u := replaceURL(url, p, true)
					pageURLTmp = append(pageURLTmp, u)
				}
				for _, pageU := range pageURLTmp {
					url := pageURLs(pageU)
					urls = append(urls, url...)
				}
			} else if tag == "plus" {
				urls = MultiPageURLs(tag, number, url)
			} else if tag == "minus" {
				urls = MultiPageURLs(tag, number, url)
			} else {
				urls = pageURLs(url)
			}
		}
	} else {
		urls = []string{}
		stat = false
		return 
	}
	return
}

// BlogCore 并行获取博客内容
func BlogCore(id int, url string, dl *DLserver) {
	log.Printf("<Task %d> - %s", id, url)
	blogRaw := GetBlogRaw(url)
	dlImg := new(DLserver)
	SaveToText(id, blogRaw, dlImg, 4)
	dl.WG.Done()
	<-dl.Gonum
}

// URLAllocate 分配url
func URLAllocate(urls []string) {
	dl := new(DLserver)
	// 启用多核
	runtime.GOMAXPROCS(runtime.NumCPU())
	// 用channel阻塞控制goroutine的数量
	dl.Gonum = make(chan string, 4)
	// 当前任务数量
	tasks := len(urls)
	dl.WG.Add(tasks)
	log.Printf("There are [%d] blogs\n", tasks)
	for id, url := range urls {
		id = id + 1
		dl.Gonum <- url
		go BlogCore(id, url, dl)
	}
	dl.WG.Wait()
}

func main() {
	// 接收地址
	tipSingle := "[URL Single Page | 按篇]\nhttp://www.keyakizaka46.com/s/k46o/diary/detail/15117?ima=0000&cd=member"
	tipPage := "[URL Page Index | 按页]\nhttp://www.keyakizaka46.com/s/k46o/diary/member/list?ima=0000&page=1&cd=member&ct=20"
	tipProfile := "[URL Profile Page | 按页]\nhttp://www.keyakizaka46.com/s/k46o/artist/20?ima=0000"
	tipNum := "[Number Type | 数字类型]\n1 [current] | +4 / -6 Next or Prev | 1-5 a range of pages\n1 [当前页或篇] | +4 / -6 当前页或篇前后 | 1-5 一个页数范围"
	fmt.Printf("A Blog URL and Number Type | 博客地址和获取方式\n")
	fmt.Printf("---------Example-----------\n%s\n\n%s\n\n%s\n\n\n%s\n--------------------------\n", tipSingle, tipPage, tipProfile, tipNum)
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("A Blog URL: ")
	data1, _, _ := reader.ReadLine()
	url, ok := urlCheck(string(data1))
	if ok {
		// 接收数量
		fmt.Print("Number Type: ")
		data2, _, _ := reader.ReadLine()
		num := string(data2)
		// 调用函数
		URLMode := URLAnalysis(url, num)
		blogURLs, stat := BlogURLsGet(URLMode)
		if stat {
			log.Println("Get Blog Info...")
			URLAllocate(blogURLs)
			fmt.Println("Mission Completed.")
		} else {
			fmt.Println("URL / Number Type invalid.")
		}
		fmt.Println("Ctrl+C to exit.")
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)
		<-c
	} else {
		fmt.Println("Url is invalid ! Ctrl+C to exit.")
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)
		<-c
	}
}
