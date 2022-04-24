![pixiv_hunter](https://github.com/ClarkQAQ/pixiv_hunter/blob/master/images/pixiv_hunter.jpeg?raw=true)


### 他是什么, 做什么的?


> PixivHunter 是一个收图工具也是一个爬虫, 可以爬取 `pixiv.net` 账户收藏的图片, 并且可以将图片自动去重下载到本地. 主要是自己日常使用和演示 `https://github.com/ClarkQAQ/gapi` 的使用方法.



##### 工作流程:


```
0. 在你能访问的任意客户端选择你心仪的插画并点击红心
1. 使用开发者工具获取PHPSESSID并通过`-session xxxxxxx`传给hunter以登录账户
1.1. 国内用户可以使用`-proxy socks5://user:pass@host:port`指定代理 (需要支持socks5代理)
1.2. Clash 用户一般是 `-proxy socks5://127.0.0.1:7891` SSR 用户一般是 `-proxy socks5://127.0.0.1:1080`
2. 然后hunter会自动登录账户, 并且获取公开收藏的插画, 然后自动下载到本地, 并把已经下载的插画移动到非公开收藏
```

### 使用方法

1. 下载 `pixiv_hunter` 到你的本地

Linux: 

```
chmod +x pixiv_hunter
pixiv_hunter -session xxxxxx -proxy socks5://user:pass@host:port
```

Windows:

```
pixiv_hunter.exe -session xxxxxx -proxy socks5://user:pass@host:port
```


> 如果下载失败会有60秒的等待时间, 如果还是失败请检查你的网络连接是否正常或者复制log提issues.