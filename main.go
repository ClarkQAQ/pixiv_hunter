package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"utilware/gjson"
	"utilware/logger"

	"github.com/ClarkQAQ/gapi"
	"github.com/ClarkQAQ/gapi/apis/pixiv"
)

var (
	flags *Flags = &Flags{}
)

type Flags struct {
	BookmarkDir            string        // 下载保存目录
	ProxyURL               string        // 代理地址
	Timeout                time.Duration // 全局超时时间
	PHPSessionID           string        // Pixiv PHP Session ID
	DownloadedBookmarkName string        // 下载后移动收藏夹名称
	DebugPrint             bool          // 是否打印调试信息
	HideTag                bool          // 是否隐藏Tag输出
}

func main() {
	logger.Info("Pixiv Hunter - Pixiv收藏夹下载工具")
	logger.Info("批量下载公开收藏夹内的插画并在下载完成后移动到非公开收藏夹并指定TAG")
	logger.Info("项目地址: https://github.com/ClarkQAQ/pixiv_hunter")

	// 读取命令行参数
	flag.StringVar(&flags.BookmarkDir, "path", "download", "收藏夹下载保存目录")
	flag.StringVar(&flags.ProxyURL, "proxy", "", "代理地址URL, 格式: socks5://user:pass@host:port")
	flag.DurationVar(&flags.Timeout, "timeout", time.Second*20, "超时时间")
	flag.StringVar(&flags.PHPSessionID, "session", os.Getenv("PIXIV_SESSION"), "Pixiv PHP Session ID")
	flag.StringVar(&flags.DownloadedBookmarkName, "bookmark", "已下载", "下载后移动未公开收藏夹Tag")
	flag.BoolVar(&flags.DebugPrint, "debug", false, "是否打印调试信息")
	flag.BoolVar(&flags.HideTag, "hide", false, "是否隐藏Tag输出")
	flag.Parse()

	// 开关打印调试信息
	if !flags.DebugPrint {
		logger.Stdout().SetLevel(logger.LevelInfo)
	}

	// 输出参数内容
	logger.Debug("Flags: %+v", flags)

	// 初始化创建文件夹
	if e := os.MkdirAll(filepath.Join(flags.BookmarkDir, "r18"), os.ModePerm); e != nil {
		logger.Fatal("创建R18目录失败: %s", e.Error())
	}
	if e := os.MkdirAll(filepath.Join(flags.BookmarkDir, "safe"), os.ModePerm); e != nil {
		logger.Fatal("创建Safe目录失败: %s", e.Error())
	}

	gapiOptions := &gapi.Options{
		Timeout: flags.Timeout,
	}

	if flags.ProxyURL != "" {
		gapiOptions.ProxyURL = flags.ProxyURL
	}

	// 创建客户端
	p, e := gapi.New(pixiv.URL, gapiOptions)
	if e != nil {
		logger.Fatal("创建客户端失败: %s", e.Error())
	}

	// 设置应用默认全局请求头
	p.SetGHeader(pixiv.GlobalHeader)

	// 尝试使用PHP SESSION ID登录
	resp, e := p.Do(pixiv.CookieLogin(flags.PHPSessionID))
	if e != nil {
		logger.Fatal("登录失败: %s", e.Error())
	}

	logger.Info("登录成功: %s", resp.Raw())

	// 获取收藏夹列表并下载
	for next := true; next; {
		e, next = getBookmarkList(p, "", "show", 100)
		if e != nil {
			logger.Error("下载收藏失败: %s", e.Error())
			logger.Warn("即将准备重试...")

			l := logger.SingleProgress(10, 60, "")
			for i := 0; i < 60; i++ {
				l.Push(1, "重试等待倒计时!")
				time.Sleep(time.Second)
			}
		}
	}

	logger.Info("下载完成!")
}

func getBookmarkList(p *gapi.Gapi, tag, rest string, limit int64) (error, bool) {
	// 获取收藏夹列表
	resp, e := p.Do(pixiv.BookmarkList(tag, rest, "zh", 0, limit))
	if e != nil {
		return fmt.Errorf("获取账户收藏失败: %s", e.Error()), true
	}

	res, e := resp.GJSON()
	if e != nil {
		return fmt.Errorf("解析账户收藏json失败: %s", e.Error()), false
	}

	array := res.Get("works").Array()

	logger.Debug("获取账户收藏成功: %d", len(array))

	for i := 0; i < len(array); i++ {
		// 下载单张插画
		if e := bookmarkDownload(p, array[i]); e != nil {
			return fmt.Errorf("下载收藏失败: %s", e.Error()), true
		}
	}

	// 判断是否还有, 有则继续获取
	total := res.Get("total").Int()
	if total-limit > 0 {
		return getBookmarkList(p, tag, rest, limit)
	}

	return nil, false
}

func bookmarkDownload(p *gapi.Gapi, value gjson.Result) error {
	defer fmt.Print("\n\n")

	artworkId := value.Get("id").Int()                                     // 插画编号
	artworkTitle := value.Get("title").String()                            // 插画标题
	artworkTags := value.Get("tags").Array()                               // 插画标签
	artworkAuthor := value.Get("userName").String()                        // 插画作者账户名
	artworkPageCount := value.Get("pageCount").Int()                       // 插画图片数量
	artworkIsR18 := strings.Contains(value.Get("tags.0").String(), "R-18") // 插画是否R18
	artworkIsDelete := artworkTitle != "-----"                             // 插画是否被删除 (后续找个更好的判断方法)

	// 插画保存路径
	bookmarkPath := filepath.Join(flags.BookmarkDir, "safe")
	if artworkIsR18 {
		bookmarkPath = filepath.Join(flags.BookmarkDir, "r18")
	}

	logger.Info("作品编号: %d, 作品标题: %s", artworkId, artworkTitle)
	if !flags.HideTag {
		logger.Info("作品标签: %v, R18: %v", artworkTags, artworkIsR18)
	}
	logger.Info("作者: %s", artworkAuthor)
	logger.Info("图片数: %d", artworkPageCount)

	// 获取作品详细的图片列表
	resp, e := p.Do(pixiv.GetIllust(artworkId, "zh"))
	if e != nil {
		return fmt.Errorf("获取作品编号: %d 详细的图片列表失败: %s", artworkId, e.Error())
	}

	// 同样解析json 获取图片列表
	res, e := resp.GJSON()
	if e != nil {
		return fmt.Errorf("解析作品编号: %d 详细的图片列表JSON失败: %s", artworkId, e.Error())
	}

	l := logger.Progress(30, artworkPageCount, "张图片")

	res.Get("body").ForEach(func(key, value gjson.Result) bool {
		// 原图图片地址
		artworkPicUrl := value.Get("urls.original").String()

		u, e := url.Parse(artworkPicUrl)
		if e != nil {
			logger.Fatal("编号: %d 原始图片URL: %s 解析URL失败(请留意接口数据变化): %s", artworkId, artworkPicUrl, e.Error())
			return true
		}

		artworkPicName := fmt.Sprintf("%d_%d%s", artworkId, key.Int(), filepath.Ext(u.Path))

		// 防止那种下载到一半然后被kill了的情况
		if s, e := os.Stat(filepath.Join(bookmarkPath, artworkPicName)); e == nil && s.Size() > 0 {
			l.Push(1, "🎊 正在跳过作品 %s 图片 %s: ", artworkTitle, artworkPicName)

			logger.Warn("编号: %d 原始图片URL: %s 已存在", artworkId, artworkPicUrl)
			return true
		}

		logger.Debug("编号: %d 原图图片地址: %s", artworkId, u.String())

		// 调用下载图片的函数下载图片
		b, e := p.Do(pixiv.Pximg(u.String()))
		if e != nil {
			logger.Warn("编号: %d 图片URL: %s 下载失败: %s", artworkId, u.String(), e.Error())
			return true
		}

		if e := ioutil.WriteFile(filepath.Join(bookmarkPath, artworkPicName),
			b.Raw(), os.ModePerm); e != nil {
			logger.Fatal("编号: %d 图片URL: %s 保存失败(请确认是否有权限或者其他问题): %s", artworkId, artworkPicUrl, e.Error())
			return true
		}

		l.Push(1, "🎉 正在下载作品: %s", artworkTitle)
		logger.Debug("编号: %d 图片URL: %s 下载成功", artworkId, artworkPicUrl)
		return true
	})

	if l.GetCurrent() >= artworkPageCount || artworkIsDelete {
		// 取消收藏
		resp, e := p.Do(pixiv.DeleteBookmark(artworkId))
		if e != nil {
			return fmt.Errorf("取消收藏请求失败: %s", e.Error())
		}
		if string(resp.Raw()) != "success" {
			return fmt.Errorf("取消收藏失败: %s", string(resp.Raw()))
		}

		// 添加收藏到非公开的收藏夹
		if artworkIsDelete {
			resp, e = p.Do(pixiv.AddBookmark(artworkId, 1, "", []string{flags.DownloadedBookmarkName}))
			if e != nil {
				return fmt.Errorf("添加收藏请求失败: %s", e.Error())
			}

			if string(resp.Raw()) != "success" {
				return fmt.Errorf("添加收藏失败: %s", string(resp.Raw()))
			}
		}
	}

	return nil
}
